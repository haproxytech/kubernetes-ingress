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
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>

#define LIB_PATH "LD_PRELOAD=/usr/local/lib/libblock_secrets.so"
#define TARGET_PATH "/usr/local/sbin/haproxy"

int main(int argc, char *argv[], char *envp[]) {
    char **newargv = malloc((argc + 1) * sizeof(char *));
    if (!newargv) {
        perror("malloc");
        return 1;
    }
    newargv[0] = (char *)TARGET_PATH;
    for (int i = 1; i < argc; i++) {
        newargv[i] = argv[i];
    }
    newargv[argc] = NULL;

    int envc = 0;
    while (environ[envc]) envc++;

    char **new_envp = malloc((envc + 2) * sizeof(char *));
    for (int i = 0; i < envc; i++) {
        new_envp[i] = environ[i];
    }
    new_envp[envc] = LIB_PATH;
    new_envp[envc + 1] = NULL;

    execve(TARGET_PATH, newargv, new_envp);

    perror("execve");
    return 1;
}
