# proxy-protocol

A simple Go web server backed by PROXY Protocol.

## How to use

```bash
    docker build -t haproxytech/proxy-protocol -f deploy/tests/images/proxy-protocol/Dockerfile deploy/tests/images/proxy-protocol
    docker run -p 8080:8080 --rm -t haproxytech/proxy-protocol
```

## Output example

```
$: curl --haproxy-protocol --http1.1 http://localhost:8080/
hello!
````

## Credits

[github.com/pires/go-proxyproto](https://github.com/pires/go-proxyproto)