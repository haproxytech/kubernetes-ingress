# Copyright 2017 The Kubernetes Authors. All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
FROM haproxy:1.8-alpine

RUN apk --no-cache add socat openssl util-linux openrc
RUN apk update && apk add bash make

ARG DUMB_INIT_SHA256=37f2c1f0372a45554f1b89924fbb134fc24c3756efaedf11e07f599494e0eff9
RUN wget --no-check-certificate -O /dumb-init https://github.com/Yelp/dumb-init/releases/download/v1.2.2/dumb-init_1.2.2_amd64 && \
	echo "$DUMB_INIT_SHA256  /dumb-init" | sha256sum -c - && \
	chmod +x /dumb-init


COPY /lbctl /usr/lbctl/
RUN mkdir /tmp/lbctl
RUN cd /usr/lbctl/ && make install
RUN ln -s /opt/lbctl/scripts/lbctl /usr/sbin/lbctl
RUN chmod +x /usr/sbin/lbctl

COPY /fs /
RUN chmod +x /etc/init.d/haproxy

ENTRYPOINT ["/dumb-init", "--", "/start.sh"]
