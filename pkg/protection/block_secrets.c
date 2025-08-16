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

#include <dlfcn.h>
#include <errno.h>
#include <fcntl.h>
#include <limits.h>
#include <stdarg.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <unistd.h>

#define PATH_MAX 4096
#define BLOCKED_PATH "/var/run/secrets/kubernetes.io/"

static char canonical_blocked[PATH_MAX] = {0};
static size_t canonical_blocked_len = 0;

__attribute__((constructor)) static void init_blocked_path() {
  if (!realpath(BLOCKED_PATH, canonical_blocked)) {
    strncpy(canonical_blocked, BLOCKED_PATH, PATH_MAX - 1);
    canonical_blocked[PATH_MAX - 1] = '\0';
  }
  canonical_blocked_len = strlen(canonical_blocked);
}

__attribute__((always_inline)) inline static int
is_blocked(const char *pathname) {
  char resolved[PATH_MAX];
  const char *target = pathname;

  if (realpath(pathname, resolved)) {
    target = resolved;
  }

  return strncmp(target, canonical_blocked, canonical_blocked_len) == 0;
}

static int (*real_open)(const char *, int, ...) = NULL;
static int (*real_open64)(const char *, int, ...) = NULL;
static FILE *(*real_fopen)(const char *, const char *) = NULL;
static FILE *(*real_fopen64)(const char *, const char *) = NULL;
static FILE *(*real_freopen)(const char *, const char *, FILE *) = NULL;
static FILE *(*real_freopen64)(const char *, const char *, FILE *) = NULL;

__attribute__((constructor)) static void init_hooks() {
  real_open = dlsym(RTLD_NEXT, "open");
  real_open64 = dlsym(RTLD_NEXT, "open64");
  real_fopen = dlsym(RTLD_NEXT, "fopen");
  real_fopen64 = dlsym(RTLD_NEXT, "fopen64");
  real_freopen = dlsym(RTLD_NEXT, "freopen");
  real_freopen64 = dlsym(RTLD_NEXT, "freopen64");
}

int open(const char *pathname, int flags, ...) {
  if (is_blocked(pathname)) {
    errno = EACCES;
    return -1;
  }

  va_list args;
  va_start(args, flags);
  int fd;
  if (flags & O_CREAT) {
    mode_t mode = (mode_t)va_arg(args, int);
    fd = real_open(pathname, flags, mode);
  } else {
    fd = real_open(pathname, flags);
  }
  va_end(args);
  return fd;
}

int open64(const char *pathname, int flags, ...) {
  if (is_blocked(pathname)) {
    errno = EACCES;
    return -1;
  }

  va_list args;
  va_start(args, flags);
  int fd;
  if (flags & O_CREAT) {
    mode_t mode = (mode_t)va_arg(args, int);
    fd = real_open64(pathname, flags, mode);
  } else {
    fd = real_open64(pathname, flags);
  }
  va_end(args);
  return fd;
}

FILE *fopen(const char *pathname, const char *mode) {
  if (is_blocked(pathname)) {
    errno = EACCES;
    return NULL;
  }
  return real_fopen(pathname, mode);
}

FILE *fopen64(const char *pathname, const char *mode) {
  if (is_blocked(pathname)) {
    errno = EACCES;
    return NULL;
  }
  return real_fopen64(pathname, mode);
}

FILE *freopen(const char *pathname, const char *mode, FILE *stream) {
  if (is_blocked(pathname)) {
    errno = EACCES;
    return NULL;
  }
  return real_freopen(pathname, mode, stream);
}

FILE *freopen64(const char *pathname, const char *mode, FILE *stream) {
  if (is_blocked(pathname)) {
    errno = EACCES;
    return NULL;
  }
  return real_freopen64(pathname, mode, stream);
}
