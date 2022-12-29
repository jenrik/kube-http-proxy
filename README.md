# kube-http-proxy

A http proxy that allows you to access `<service_name>.<namespace>.svc.cluster.local` addresses via `kube-apiserver`.

## Requirements

 * `kubectl`
 * A kubernetes configuration with permission to run `kubectl proxy`

## Credits

The proxy code is based on https://github.com/pouriya73/HTTP-Proxy-server---GOlang