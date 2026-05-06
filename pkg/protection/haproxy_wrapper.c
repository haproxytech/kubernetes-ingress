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

#define _GNU_SOURCE
#include <errno.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

#define LIB_PATH "/usr/local/lib/libblock_secrets.so"
#define TARGET_PATH "/usr/local/sbin/haproxy"

int main(int argc, char *argv[]) {
    int n = argc > 0 ? argc : 1;
    char **newargv = malloc((n + 1) * sizeof(char *));
    if (!newargv) {
        perror("malloc");
        return EXIT_FAILURE;
    }
    newargv[0] = (char *)TARGET_PATH;
    for (int i = 1; i < n; i++) {
        newargv[i] = argv[i];
    }
    newargv[n] = NULL;

    if (setenv("LD_PRELOAD", LIB_PATH, 1) != 0) {
        perror("setenv LD_PRELOAD");
        free(newargv);
        return EXIT_FAILURE;
    }

    execv(TARGET_PATH, newargv);

    fprintf(stderr, "execv %s: %s\n", TARGET_PATH, strerror(errno));
    free(newargv);
    return EXIT_FAILURE;
}
