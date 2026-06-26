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

/* Behavioural tests for libblock_secrets.so, run preloaded by
   block_secrets_test.sh. Regression coverage for issue #818:
     - content reads (open/fopen) into the protected dir stay blocked;
     - metadata calls (stat/access) are no longer hooked;
     - LFS64 aliases (open64) must not return ENOSYS on musl. */

#define _GNU_SOURCE
#include <dlfcn.h>
#include <errno.h>
#include <fcntl.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <unistd.h>

static int failures = 0;

static void check(int cond, const char *what) {
  printf("  [%s] %s\n", cond ? "PASS" : "FAIL", what);
  if (!cond) {
    failures++;
  }
}

int main(void) {
  const char *blocked_file = getenv("TEST_BLOCKED_FILE"); /* inside protected dir */
  const char *normal_file = getenv("TEST_NORMAL_FILE");   /* outside protected dir */
  struct stat st;
  int fd;
  FILE *fp;

  printf("== security: content reads into the protected dir stay blocked ==\n");
  errno = 0;
  fd = open(blocked_file, O_RDONLY);
  check(fd == -1 && errno == EACCES, "open(blocked) is denied with EACCES");
  if (fd != -1) {
    close(fd);
  }
  errno = 0;
  fp = fopen(blocked_file, "r");
  check(fp == NULL && errno == EACCES, "fopen(blocked) is denied with EACCES");
  if (fp) {
    fclose(fp);
  }

  printf("== regression #818: metadata-only calls are no longer intercepted ==\n");
  errno = 0;
  check(stat(blocked_file, &st) == 0, "stat(blocked) succeeds (not hooked)");
  errno = 0;
  check(access(blocked_file, F_OK) == 0, "access(blocked) succeeds (not hooked)");

  printf("== regression #818: LFS64 aliases must not return ENOSYS ==\n");
  int (*p_open64)(const char *, int, ...) = (void *)dlsym(RTLD_DEFAULT, "open64");
  if (p_open64) {
    errno = 0;
    fd = p_open64(normal_file, O_RDONLY);
    check(fd >= 0, "open64(normal) succeeds (no ENOSYS)");
    if (fd >= 0) {
      close(fd);
    }
  } else {
    printf("  [SKIP] open64 symbol absent\n");
  }

  printf("== sanity: normal files remain accessible ==\n");
  errno = 0;
  fd = open(normal_file, O_RDONLY);
  check(fd >= 0, "open(normal) succeeds");
  if (fd >= 0) {
    close(fd);
  }

  printf("\n%s (%d failure(s))\n", failures ? "TESTS FAILED" : "TESTS PASSED",
         failures);
  return failures ? 1 : 0;
}
