# http-echo

A simple golang HTTP/S server that echoes back request attributes to the client in JSON formats.
By default the certificate CN is the hostname of the machine where the program is running.

Listens on four ports: HTTP (8888), HTTPS (8443), HTTP behind PROXY protocol (8080) and UDP (8090).
The PROXY port rejects connections that do not send the PROXY header.
The UDP port answers every datagram with a JSON echo of its origin and payload.

## How to use

```bash
    docker build -t haproxytech/http-echo -f deploy/tests/images/http-echo/Dockerfile deploy/tests/images/http-echo
    docker run -p 8888:8888 -p 8443:8443 -p 8080:8080 -p 8090:8090/udp --rm -t haproxytech/http-echo
```

## PROXY protocol example

```bash
curl --haproxy-protocol http://localhost:8080/
```

## UDP example

```bash
echo -n "ping" | nc -u -w1 localhost 8090
```

## Output example

```bash
curl -b "test=bar" -k https://localhost:8443/path\?a\=foo1\&b\=foo2
````
```json
{
  "http": {
    "cookies": [
      "test=bar"
    ],
    "headers": {
      "Accept": "*/*",
      "Cookie": "test=bar",
      "User-Agent": "curl/7.70.0"
    },
    "host": "localhost:8443",
    "method": "GET",
    "path": "/path",
    "protocol": "HTTP/2.0",
    "query": "a=foo1\u0026b=foo2",
    "raw": "GET /path?a=foo1\u0026b=foo2 HTTP/1.1\r\nHost: localhost:8443\r\nUser-Agent: curl/7.70.0\r\nAccept: */*\r\nCookie: test=bar\r\n\r\n"
  },
  "os": {
    "hostname": "traktour"
  },
  "tcp": {
    "ip": "[::1]",
    "port": "53364"
  },
  "tls": {
    "cipher": "TLS_AES_128_GCM_SHA256",
    "sni": "localhost"
  }
}
```


## Credits

[mendhak/docker-http-https-echo](https://github.com/mendhak/docker-http-https-echo)
[github.com/pires/go-proxyproto](https://github.com/pires/go-proxyproto)
