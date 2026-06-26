/* Copyright 2025 HAProxy Technologies LLC                                    */
/*                                                                            */
/* Licensed under the Apache License, Version 2.0 (the "License");            */
/* you may not use this file except in compliance with the License.           */
/* You may obtain a copy of the License at                                    */
/*                                                                            */
/*    http://www.apache.org/licenses/LICENSE-2.0                              */
/*                                                                            */
/* Unless required by applicable law or agreed to in writing, software        */
/* distributed under the License is distributed on an "AS IS" BASIS,          */
/* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.   */
/* See the License for the specific language governing permissions and        */
/* limitations under the License.                                             */

/* LD_PRELOAD shim that denies HAProxy access to the Kubernetes service-account
   token dir (default /var/run/secrets/kubernetes.io). Interposes the libc
   calls that read or mutate a path and returns EACCES for anything resolving
   inside it. Metadata-only calls (stat/access/readlink) are deliberately NOT
   hooked so they stay off HAProxy's hot path (issue #818). */

#define _XOPEN_SOURCE 700

#include <dlfcn.h>
#include <errno.h>
#include <fcntl.h>
#include <limits.h>
#include <stdarg.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <sys/time.h>
#include <sys/types.h>
#include <time.h>
#include <unistd.h>
#include <utime.h>

/* Forward-declared to avoid _GNU_SOURCE, which on musl macro-aliases the
   *64 stat names and would collide with our hook symbols. */
struct file_handle;

#ifndef PATH_MAX
#define PATH_MAX 4096
#endif

/* Default protected dir; env-overridable. Unresolvable path → no-op. */
#define BLOCKED_PATH_DEFAULT "/var/run/secrets/kubernetes.io"
#define BLOCKED_PATH_ENV "BLOCK_SECRETS_PATH"

#define HOOK_VISIBLE __attribute__((visibility("default")))

/* Both textual and canonical prefix; fast-path requires both to miss so
   symlinked configs don't bypass. canonical_blocked_len == 0 = no-op. */
static char canonical_blocked[PATH_MAX] = {0};
static size_t canonical_blocked_len = 0;
static char text_blocked[PATH_MAX] = {0};
static size_t text_blocked_len = 0;
static int text_canonical_same = 1;

/* Recursion guard: realpath() resolves paths via the public open() symbol,
   which our hook would otherwise re-enter. */
static __thread int in_is_blocked = 0;

/* Prefix match with separator boundary so /foo doesn't match /foobar. */
static inline int path_has_prefix(const char *path, size_t plen,
                                  const char *pre, size_t prelen) {
  if (plen < prelen) {
    return 0;
  }
  if (memcmp(path, pre, prelen) != 0) {
    return 0;
  }
  return plen == prelen || path[prelen] == '/';
}

/* Absolute, no //, /./, /../, or trailing /.//.. — lexical compare is
   then equivalent to realpath. Trailing single / is fine. */
static inline int is_simple_absolute(const char *p, size_t len) {
  if (len == 0 || p[0] != '/') {
    return 0;
  }
  for (size_t i = 0; i < len; i++) {
    if (p[i] != '/') {
      continue;
    }
    if (i + 1 < len && p[i + 1] == '/') {
      return 0;
    }
    if (i + 1 < len && p[i + 1] == '.') {
      if (i + 2 == len || p[i + 2] == '/') {
        return 0;
      }
      if (p[i + 2] == '.' && (i + 3 == len || p[i + 3] == '/')) {
        return 0;
      }
    }
  }
  return 1;
}

/* Fallback when realpath() fails (typically: leaf doesn't exist on a
   create). Canonicalizes the parent and re-attaches the leaf. `out`
   doubles as the realpath() workspace to save a stack frame. */
__attribute__((cold)) static int
canonicalize_via_parent(const char *pathname, size_t plen, char *out,
                        size_t *out_len) {
  if (plen == 0) {
    return 0;
  }
  size_t i = plen;
  while (i > 0 && pathname[i - 1] != '/') {
    i--;
  }
  if (i == 0) {
    return 0; /* relative single component — indeterminate */
  }
  size_t parent_len = i - 1;
  if (parent_len == 0) {
    parent_len = 1; /* root */
  }
  if (parent_len >= PATH_MAX) {
    return 0;
  }

  char parent[PATH_MAX];
  memcpy(parent, pathname, parent_len);
  parent[parent_len] = '\0';

  if (realpath(parent, out) == NULL) {
    return 0;
  }
  size_t presolved_len = strlen(out);

  if (i == plen) {
    *out_len = presolved_len; /* trailing slash → leaf empty */
    return 1;
  }

  const char *leaf = pathname + i;
  size_t leaf_len = plen - i;
  int need_sep = (presolved_len == 0 || out[presolved_len - 1] != '/');
  if (presolved_len + (size_t)(need_sep ? 1 : 0) + leaf_len + 1 > PATH_MAX) {
    return 0;
  }
  size_t off = presolved_len;
  if (need_sep) {
    out[off++] = '/';
  }
  memcpy(out + off, leaf, leaf_len);
  off += leaf_len;
  out[off] = '\0';
  *out_len = off;
  return 1;
}

/* Returns 1 iff pathname resolves into the protected directory.
   Indeterminate inputs return 0; the library must never break unrelated I/O.
   Limitations: TOCTOU; *at(dirfd, relative-pathname) bypass; lexical
   fast-path assumes no rogue symlinks redirect into the protected dir. */
static int is_blocked(const char *pathname) {
  if (canonical_blocked_len == 0) {
    return 0;
  }
  if (in_is_blocked) {
    return 0;
  }
  if (pathname == NULL) {
    return 0;
  }
  size_t plen = strlen(pathname);
  if (plen == 0 || plen >= PATH_MAX) {
    return 0;
  }

  /* Fast-path: skip realpath when both prefixes obviously miss. */
  if (is_simple_absolute(pathname, plen) &&
      !path_has_prefix(pathname, plen, text_blocked, text_blocked_len) &&
      (text_canonical_same ||
       !path_has_prefix(pathname, plen, canonical_blocked,
                        canonical_blocked_len))) {
    return 0;
  }

  /* Slow-path: realpath clobbers errno; preserve. */
  in_is_blocked = 1;
  int saved_errno = errno;
  char resolved[PATH_MAX];
  int result = 0;

  if (realpath(pathname, resolved) != NULL) {
    result = path_has_prefix(resolved, strlen(resolved), canonical_blocked,
                             canonical_blocked_len);
  } else {
    size_t full_len;
    if (canonicalize_via_parent(pathname, plen, resolved, &full_len)) {
      result = path_has_prefix(resolved, full_len, canonical_blocked,
                               canonical_blocked_len);
    }
  }

  errno = saved_errno;
  in_is_blocked = 0;
  return result;
}

static inline int path_blocked(const char *pathname) {
  return pathname != NULL && is_blocked(pathname);
}

static inline int either_blocked(const char *p1, const char *p2) {
  return path_blocked(p1) || path_blocked(p2);
}

static int (*real_open)(const char *, int, ...) = NULL;
static int (*real_open64)(const char *, int, ...) = NULL;
static FILE *(*real_fopen)(const char *, const char *) = NULL;
static FILE *(*real_fopen64)(const char *, const char *) = NULL;
static FILE *(*real_freopen)(const char *, const char *, FILE *) = NULL;
static FILE *(*real_freopen64)(const char *, const char *, FILE *) = NULL;
static int (*real_creat)(const char *, mode_t) = NULL;
static int (*real_creat64)(const char *, mode_t) = NULL;
static int (*real_openat)(int, const char *, int, ...) = NULL;
static int (*real_openat64)(int, const char *, int, ...) = NULL;
static int (*real_name_to_handle_at)(int, const char *, struct file_handle *,
                                     int *, int) = NULL;
static int (*real_mkdir)(const char *, mode_t) = NULL;
static int (*real_mkdirat)(int, const char *, mode_t) = NULL;
static int (*real_rmdir)(const char *) = NULL;
static int (*real_unlink)(const char *) = NULL;
static int (*real_unlinkat)(int, const char *, int) = NULL;
static int (*real_truncate)(const char *, off_t) = NULL;
/* int64_t == off64_t on every supported ABI; avoids _LARGEFILE64_SOURCE. */
static int (*real_truncate64)(const char *, int64_t) = NULL;
static int (*real_chmod)(const char *, mode_t) = NULL;
static int (*real_fchmodat)(int, const char *, mode_t, int) = NULL;
static int (*real_chown)(const char *, uid_t, gid_t) = NULL;
static int (*real_fchownat)(int, const char *, uid_t, gid_t, int) = NULL;
static int (*real_lchown)(const char *, uid_t, gid_t) = NULL;
static int (*real_utime)(const char *, const struct utimbuf *) = NULL;
static int (*real_utimes)(const char *, const struct timeval *) = NULL;
static int (*real_utimensat)(int, const char *, const struct timespec *,
                             int) = NULL;
static int (*real_mknod)(const char *, mode_t, dev_t) = NULL;
static int (*real_mknodat)(int, const char *, mode_t, dev_t) = NULL;
static int (*real_rename)(const char *, const char *) = NULL;
static int (*real_renameat)(int, const char *, int, const char *) = NULL;
static int (*real_renameat2)(int, const char *, int, const char *,
                             unsigned int) = NULL;
static int (*real_link)(const char *, const char *) = NULL;
static int (*real_linkat)(int, const char *, int, const char *, int) = NULL;
static int (*real_symlink)(const char *, const char *) = NULL;
static int (*real_symlinkat)(const char *, int, const char *) = NULL;

/* Priority 101 (0–100 reserved). dlsym before realpath so any internal
   hook recursion finds populated pointers. */
__attribute__((constructor(101), cold)) static void block_secrets_init(void) {
  real_open = dlsym(RTLD_NEXT, "open");
  real_open64 = dlsym(RTLD_NEXT, "open64");
  real_fopen = dlsym(RTLD_NEXT, "fopen");
  real_fopen64 = dlsym(RTLD_NEXT, "fopen64");
  real_freopen = dlsym(RTLD_NEXT, "freopen");
  real_freopen64 = dlsym(RTLD_NEXT, "freopen64");
  real_creat = dlsym(RTLD_NEXT, "creat");
  real_creat64 = dlsym(RTLD_NEXT, "creat64");
  real_openat = dlsym(RTLD_NEXT, "openat");
  real_openat64 = dlsym(RTLD_NEXT, "openat64");
  real_name_to_handle_at = dlsym(RTLD_NEXT, "name_to_handle_at");
  real_mkdir = dlsym(RTLD_NEXT, "mkdir");
  real_mkdirat = dlsym(RTLD_NEXT, "mkdirat");
  real_rmdir = dlsym(RTLD_NEXT, "rmdir");
  real_unlink = dlsym(RTLD_NEXT, "unlink");
  real_unlinkat = dlsym(RTLD_NEXT, "unlinkat");
  real_truncate = dlsym(RTLD_NEXT, "truncate");
  real_truncate64 = dlsym(RTLD_NEXT, "truncate64");
  real_chmod = dlsym(RTLD_NEXT, "chmod");
  real_fchmodat = dlsym(RTLD_NEXT, "fchmodat");
  real_chown = dlsym(RTLD_NEXT, "chown");
  real_fchownat = dlsym(RTLD_NEXT, "fchownat");
  real_lchown = dlsym(RTLD_NEXT, "lchown");
  real_utime = dlsym(RTLD_NEXT, "utime");
  real_utimes = dlsym(RTLD_NEXT, "utimes");
  real_utimensat = dlsym(RTLD_NEXT, "utimensat");
  real_mknod = dlsym(RTLD_NEXT, "mknod");
  real_mknodat = dlsym(RTLD_NEXT, "mknodat");
  real_rename = dlsym(RTLD_NEXT, "rename");
  real_renameat = dlsym(RTLD_NEXT, "renameat");
  real_renameat2 = dlsym(RTLD_NEXT, "renameat2");
  real_link = dlsym(RTLD_NEXT, "link");
  real_linkat = dlsym(RTLD_NEXT, "linkat");
  real_symlink = dlsym(RTLD_NEXT, "symlink");
  real_symlinkat = dlsym(RTLD_NEXT, "symlinkat");

  /* musl exports no distinct LFS64 symbols, so the dlsym()s above leave these
     NULL. off_t is 64-bit on every supported ABI, so *64 == base: alias them.
     Otherwise the exported *64 hooks would return ENOSYS (issue #818). */
  if (real_open64 == NULL) {
    real_open64 = real_open;
  }
  if (real_fopen64 == NULL) {
    real_fopen64 = real_fopen;
  }
  if (real_freopen64 == NULL) {
    real_freopen64 = real_freopen;
  }
  if (real_creat64 == NULL) {
    real_creat64 = real_creat;
  }
  if (real_openat64 == NULL) {
    real_openat64 = real_openat;
  }
  if (real_truncate64 == NULL) {
    real_truncate64 = (int (*)(const char *, int64_t))real_truncate;
  }

  const char *src = getenv(BLOCKED_PATH_ENV);
  if (src == NULL) {
    src = BLOCKED_PATH_DEFAULT;
  }
  if (realpath(src, canonical_blocked) != NULL) {
    size_t can_len = strlen(canonical_blocked);
    while (can_len > 1 && canonical_blocked[can_len - 1] == '/') {
      canonical_blocked[--can_len] = '\0';
    }
    canonical_blocked_len = can_len;

    size_t src_len = strlen(src);
    if (src_len >= PATH_MAX) {
      src_len = PATH_MAX - 1;
    }
    memcpy(text_blocked, src, src_len);
    text_blocked[src_len] = '\0';
    while (src_len > 1 && text_blocked[src_len - 1] == '/') {
      text_blocked[--src_len] = '\0';
    }
    text_blocked_len = src_len;

    text_canonical_same =
        (text_blocked_len == canonical_blocked_len &&
         memcmp(text_blocked, canonical_blocked, canonical_blocked_len) == 0);
  } else {
    canonical_blocked[0] = '\0';
    canonical_blocked_len = 0;
    text_blocked[0] = '\0';
    text_blocked_len = 0;
    text_canonical_same = 1;
  }
}

/* Lazy dlsym for pre-init or absent symbols; ENOSYS only if truly missing. */
#define HOOK_GUARD(fp, retval)                                                 \
  do {                                                                         \
    if (__builtin_expect(!(fp), 0)) {                                          \
      (fp) = dlsym(RTLD_NEXT, __func__);                                       \
      if (!(fp)) {                                                             \
        errno = ENOSYS;                                                        \
        return (retval);                                                       \
      }                                                                        \
    }                                                                          \
  } while (0)

/* open*() always reads mode_t — benign on AMD64 SysV and AArch64 AAPCS;
   revisit for s390x/ppc64le/RISC-V. Kernel discards mode when unused. */

/* ============================ Hooks ============================ */

HOOK_VISIBLE int open(const char *pathname, int flags, ...) {
  if (path_blocked(pathname)) {
    errno = EACCES;
    return -1;
  }
  HOOK_GUARD(real_open, -1);

  va_list args;
  va_start(args, flags);
  mode_t mode = (mode_t)va_arg(args, int);
  va_end(args);
  return real_open(pathname, flags, mode);
}

HOOK_VISIBLE int open64(const char *pathname, int flags, ...) {
  if (path_blocked(pathname)) {
    errno = EACCES;
    return -1;
  }
  HOOK_GUARD(real_open64, -1);

  va_list args;
  va_start(args, flags);
  mode_t mode = (mode_t)va_arg(args, int);
  va_end(args);
  return real_open64(pathname, flags, mode);
}

HOOK_VISIBLE FILE *fopen(const char *pathname, const char *mode) {
  if (path_blocked(pathname)) {
    errno = EACCES;
    return NULL;
  }
  HOOK_GUARD(real_fopen, NULL);
  return real_fopen(pathname, mode);
}

HOOK_VISIBLE FILE *fopen64(const char *pathname, const char *mode) {
  if (path_blocked(pathname)) {
    errno = EACCES;
    return NULL;
  }
  HOOK_GUARD(real_fopen64, NULL);
  return real_fopen64(pathname, mode);
}

HOOK_VISIBLE FILE *freopen(const char *pathname, const char *mode,
                           FILE *stream) {
  if (path_blocked(pathname)) {
    errno = EACCES;
    return NULL;
  }
  HOOK_GUARD(real_freopen, NULL);
  return real_freopen(pathname, mode, stream);
}

HOOK_VISIBLE FILE *freopen64(const char *pathname, const char *mode,
                             FILE *stream) {
  if (path_blocked(pathname)) {
    errno = EACCES;
    return NULL;
  }
  HOOK_GUARD(real_freopen64, NULL);
  return real_freopen64(pathname, mode, stream);
}

HOOK_VISIBLE int creat(const char *pathname, mode_t mode) {
  if (path_blocked(pathname)) {
    errno = EACCES;
    return -1;
  }
  HOOK_GUARD(real_creat, -1);
  return real_creat(pathname, mode);
}

HOOK_VISIBLE int creat64(const char *pathname, mode_t mode) {
  if (path_blocked(pathname)) {
    errno = EACCES;
    return -1;
  }
  HOOK_GUARD(real_creat64, -1);
  return real_creat64(pathname, mode);
}

HOOK_VISIBLE int openat(int dirfd, const char *pathname, int flags, ...) {
  if (path_blocked(pathname)) {
    errno = EACCES;
    return -1;
  }
  HOOK_GUARD(real_openat, -1);

  va_list args;
  va_start(args, flags);
  mode_t mode = (mode_t)va_arg(args, int);
  va_end(args);
  return real_openat(dirfd, pathname, flags, mode);
}

HOOK_VISIBLE int openat64(int dirfd, const char *pathname, int flags, ...) {
  if (path_blocked(pathname)) {
    errno = EACCES;
    return -1;
  }
  HOOK_GUARD(real_openat64, -1);

  va_list args;
  va_start(args, flags);
  mode_t mode = (mode_t)va_arg(args, int);
  va_end(args);
  return real_openat64(dirfd, pathname, flags, mode);
}

HOOK_VISIBLE int name_to_handle_at(int dirfd, const char *pathname,
                                   struct file_handle *handle, int *mount_id,
                                   int flags) {
  if (path_blocked(pathname)) {
    errno = EACCES;
    return -1;
  }
  HOOK_GUARD(real_name_to_handle_at, -1);
  return real_name_to_handle_at(dirfd, pathname, handle, mount_id, flags);
}

HOOK_VISIBLE int mkdir(const char *pathname, mode_t mode) {
  if (path_blocked(pathname)) {
    errno = EACCES;
    return -1;
  }
  HOOK_GUARD(real_mkdir, -1);
  return real_mkdir(pathname, mode);
}

HOOK_VISIBLE int mkdirat(int dirfd, const char *pathname, mode_t mode) {
  if (path_blocked(pathname)) {
    errno = EACCES;
    return -1;
  }
  HOOK_GUARD(real_mkdirat, -1);
  return real_mkdirat(dirfd, pathname, mode);
}

HOOK_VISIBLE int rmdir(const char *pathname) {
  if (path_blocked(pathname)) {
    errno = EACCES;
    return -1;
  }
  HOOK_GUARD(real_rmdir, -1);
  return real_rmdir(pathname);
}

HOOK_VISIBLE int unlink(const char *pathname) {
  if (path_blocked(pathname)) {
    errno = EACCES;
    return -1;
  }
  HOOK_GUARD(real_unlink, -1);
  return real_unlink(pathname);
}

HOOK_VISIBLE int unlinkat(int dirfd, const char *pathname, int flags) {
  if (path_blocked(pathname)) {
    errno = EACCES;
    return -1;
  }
  HOOK_GUARD(real_unlinkat, -1);
  return real_unlinkat(dirfd, pathname, flags);
}

HOOK_VISIBLE int truncate(const char *pathname, off_t length) {
  if (path_blocked(pathname)) {
    errno = EACCES;
    return -1;
  }
  HOOK_GUARD(real_truncate, -1);
  return real_truncate(pathname, length);
}

HOOK_VISIBLE int truncate64(const char *pathname, int64_t length) {
  if (path_blocked(pathname)) {
    errno = EACCES;
    return -1;
  }
  HOOK_GUARD(real_truncate64, -1);
  return real_truncate64(pathname, length);
}

HOOK_VISIBLE int chmod(const char *pathname, mode_t mode) {
  if (path_blocked(pathname)) {
    errno = EACCES;
    return -1;
  }
  HOOK_GUARD(real_chmod, -1);
  return real_chmod(pathname, mode);
}

HOOK_VISIBLE int fchmodat(int dirfd, const char *pathname, mode_t mode,
                          int flags) {
  if (path_blocked(pathname)) {
    errno = EACCES;
    return -1;
  }
  HOOK_GUARD(real_fchmodat, -1);
  return real_fchmodat(dirfd, pathname, mode, flags);
}

HOOK_VISIBLE int chown(const char *pathname, uid_t owner, gid_t group) {
  if (path_blocked(pathname)) {
    errno = EACCES;
    return -1;
  }
  HOOK_GUARD(real_chown, -1);
  return real_chown(pathname, owner, group);
}

HOOK_VISIBLE int fchownat(int dirfd, const char *pathname, uid_t owner,
                          gid_t group, int flags) {
  if (path_blocked(pathname)) {
    errno = EACCES;
    return -1;
  }
  HOOK_GUARD(real_fchownat, -1);
  return real_fchownat(dirfd, pathname, owner, group, flags);
}

HOOK_VISIBLE int lchown(const char *pathname, uid_t owner, gid_t group) {
  if (path_blocked(pathname)) {
    errno = EACCES;
    return -1;
  }
  HOOK_GUARD(real_lchown, -1);
  return real_lchown(pathname, owner, group);
}

HOOK_VISIBLE int utime(const char *pathname, const struct utimbuf *times) {
  if (path_blocked(pathname)) {
    errno = EACCES;
    return -1;
  }
  HOOK_GUARD(real_utime, -1);
  return real_utime(pathname, times);
}

HOOK_VISIBLE int utimes(const char *pathname, const struct timeval times[2]) {
  if (path_blocked(pathname)) {
    errno = EACCES;
    return -1;
  }
  HOOK_GUARD(real_utimes, -1);
  return real_utimes(pathname, times);
}

HOOK_VISIBLE int utimensat(int dirfd, const char *pathname,
                           const struct timespec times[2], int flags) {
  if (path_blocked(pathname)) {
    errno = EACCES;
    return -1;
  }
  HOOK_GUARD(real_utimensat, -1);
  return real_utimensat(dirfd, pathname, times, flags);
}

HOOK_VISIBLE int mknod(const char *pathname, mode_t mode, dev_t dev) {
  if (path_blocked(pathname)) {
    errno = EACCES;
    return -1;
  }
  HOOK_GUARD(real_mknod, -1);
  return real_mknod(pathname, mode, dev);
}

HOOK_VISIBLE int mknodat(int dirfd, const char *pathname, mode_t mode,
                         dev_t dev) {
  if (path_blocked(pathname)) {
    errno = EACCES;
    return -1;
  }
  HOOK_GUARD(real_mknodat, -1);
  return real_mknodat(dirfd, pathname, mode, dev);
}

HOOK_VISIBLE int rename(const char *oldpath, const char *newpath) {
  if (either_blocked(oldpath, newpath)) {
    errno = EACCES;
    return -1;
  }
  HOOK_GUARD(real_rename, -1);
  return real_rename(oldpath, newpath);
}

HOOK_VISIBLE int renameat(int olddirfd, const char *oldpath, int newdirfd,
                          const char *newpath) {
  if (either_blocked(oldpath, newpath)) {
    errno = EACCES;
    return -1;
  }
  HOOK_GUARD(real_renameat, -1);
  return real_renameat(olddirfd, oldpath, newdirfd, newpath);
}

HOOK_VISIBLE int renameat2(int olddirfd, const char *oldpath, int newdirfd,
                           const char *newpath, unsigned int flags) {
  if (either_blocked(oldpath, newpath)) {
    errno = EACCES;
    return -1;
  }
  /* No renameat2 on musl: flags==0 is plain renameat(); a non-zero flag is
     genuinely unsupported, so ENOSYS is then the correct answer. */
  if (real_renameat2 == NULL) {
    if (flags == 0 && real_renameat != NULL) {
      return real_renameat(olddirfd, oldpath, newdirfd, newpath);
    }
    errno = ENOSYS;
    return -1;
  }
  return real_renameat2(olddirfd, oldpath, newdirfd, newpath, flags);
}

HOOK_VISIBLE int link(const char *oldpath, const char *newpath) {
  if (either_blocked(oldpath, newpath)) {
    errno = EACCES;
    return -1;
  }
  HOOK_GUARD(real_link, -1);
  return real_link(oldpath, newpath);
}

HOOK_VISIBLE int linkat(int olddirfd, const char *oldpath, int newdirfd,
                        const char *newpath, int flags) {
  if (either_blocked(oldpath, newpath)) {
    errno = EACCES;
    return -1;
  }
  HOOK_GUARD(real_linkat, -1);
  return real_linkat(olddirfd, oldpath, newdirfd, newpath, flags);
}

HOOK_VISIBLE int symlink(const char *target, const char *linkpath) {
  if (either_blocked(target, linkpath)) {
    errno = EACCES;
    return -1;
  }
  HOOK_GUARD(real_symlink, -1);
  return real_symlink(target, linkpath);
}

HOOK_VISIBLE int symlinkat(const char *target, int newdirfd,
                           const char *linkpath) {
  if (either_blocked(target, linkpath)) {
    errno = EACCES;
    return -1;
  }
  HOOK_GUARD(real_symlinkat, -1);
  return real_symlinkat(target, newdirfd, linkpath);
}
