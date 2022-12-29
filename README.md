# kube-http-proxy

A http proxy that allows you to access `<service_name>.<namespace>.svc.cluster.local` addresses via `kube-apiserver`.

## Requirements

 * `kubectl`
 * A kubernetes configuration with permission to run `kubectl proxy`

## Gotchas

It is not possible send the `Authorization` header through the proxy because `kubectl proxy` removes the `Authorzation` header. See https://github.com/kubernetes/kubernetes/issues/38775

## Credits

The proxy code is based on https://github.com/pouriya73/HTTP-Proxy-server---GOlang