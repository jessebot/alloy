---
canonical: https://grafana.com/docs/alloy/latest/reference/components/remote/remote.kubernetes.secret/
aliases:
  - ../remote.kubernetes.secret/ # /docs/alloy/latest/reference/components/remote.kubernetes.secret/
description: Learn about remote.kubernetes.secret
labels:
  stage: general-availability
  products:
    - oss
title: remote.kubernetes.secret
---

# `remote.kubernetes.secret`

`remote.kubernetes.secret` reads a Secret from the Kubernetes API server and exposes its data for other components to consume.

A common use case for this is loading credentials or other information from secrets that aren't already mounted into the {{< param "PRODUCT_NAME" >}} Pod at deployment time.

## Usage

```alloy
remote.kubernetes.secret "<LABEL>" {
  namespace = "<NAMESPACE_OF_SECRET>"
  name = "<NAME_OF_SECRET>"
}
```

## Arguments

You can use the following arguments with `remote.kubernetes.secret`:

| Name             | Type       | Description                                         | Default | Required |
| ---------------- | ---------- | --------------------------------------------------- | ------- | -------- |
| `name`           | `string`   | Name of the Kubernetes Secret                       |         | yes      |
| `namespace`      | `string`   | Kubernetes namespace containing the desired Secret. |         | yes      |
| `poll_frequency` | `duration` | Frequency to poll the Kubernetes API.               | `"1m"`  | no       |
| `poll_timeout`   | `duration` | Timeout when polling the Kubernetes API.            | `"15s"` | no       |

When this component performs a poll operation, it requests the Secret data from the Kubernetes API.
A poll is triggered by the following:

* When the component first loads.
* Every time the component's arguments get re-evaluated.
* At the frequency specified by the `poll_frequency` argument.

Any error while polling will mark the component as unhealthy.
After a successful poll, all data is exported with the same field names as the source Secret.

## Blocks

You can use the following blocks with `remote.kubernetes.secret`:

| Block                                            | Description                                                  | Required |
| ------------------------------------------------ | ------------------------------------------------------------ | -------- |
| [`client`][client]                               | Configures Kubernetes client used to find Secrets.           | no       |
| `client` > [`authorization`][authorization]      | Configure generic authorization to the Kubernetes API.       | no       |
| `client` > [`basic_auth`][basic_auth]            | Configure basic authentication to the Kubernetes API.        | no       |
| `client` > [`oauth2`][oauth2]                    | Configure OAuth2 for authenticating to the Kubernetes API.   | no       |
| `client` > `oauth2` > [`tls_config`][tls_config] | Configure TLS settings for connecting to the Kubernetes API. | no       |
| `client` > [`tls_config`][tls_config]            | Configure TLS settings for connecting to the Kubernetes API. | no       |

The > symbol indicates deeper levels of nesting.
For example, `client` > `basic_auth` refers to a `basic_auth` block defined inside a `client` block.

[client]: #client
[authorization]: #authorization
[basic_auth]: #basic_auth
[oauth2]: #oauth2
[tls_config]: #tls_config

### `client`

The `client` block configures the Kubernetes client used to discover Secrets.
If the `client` block isn't provided, the default in-cluster configuration with the service account of the running {{< param "PRODUCT_NAME" >}} Pod is used.

The following arguments are supported:

| Name                     | Type                | Description                                                                                      | Default | Required |
| ------------------------ | ------------------- | ------------------------------------------------------------------------------------------------ | ------- | -------- |
| `api_server`             | `string`            | URL of the Kubernetes API server.                                                                |         | no       |
| `kubeconfig_file`        | `string`            | Path of the `kubeconfig` file to use for connecting to Kubernetes.                               |         | no       |
| `bearer_token_file`      | `string`            | File containing a bearer token to authenticate with.                                             |         | no       |
| `bearer_token`           | `secret`            | Bearer token to authenticate with.                                                               |         | no       |
| `enable_http2`           | `bool`              | Whether HTTP2 is supported for requests.                                                         | `true`  | no       |
| `follow_redirects`       | `bool`              | Whether redirects returned by the server should be followed.                                     | `true`  | no       |
| `http_headers`           | `map(list(secret))` | Custom HTTP headers to be sent along with each request. The map key is the header name.          |         | no       |
| `proxy_url`              | `string`            | HTTP proxy to send requests through.                                                             |         | no       |
| `no_proxy`               | `string`            | Comma-separated list of IP addresses, CIDR notations, and domain names to exclude from proxying. |         | no       |
| `proxy_from_environment` | `bool`              | Use the proxy URL indicated by environment variables.                                            | `false` | no       |
| `proxy_connect_header`   | `map(list(secret))` | Specifies headers to send to proxies during CONNECT requests.                                    |         | no       |

 At most, one of the following can be provided:

* [`authorization`][authorization] block
* [`basic_auth`][basic_auth] block
* [`bearer_token_file`][client] argument
* [`bearer_token`][client] argument
* [`oauth2`][oauth2] block

{{< docs/shared lookup="reference/components/http-client-proxy-config-description.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `authorization`

{{< docs/shared lookup="reference/components/authorization-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `basic_auth`

{{< docs/shared lookup="reference/components/basic-auth-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `oauth2`

{{< docs/shared lookup="reference/components/oauth2-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `tls_config`

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name   | Type          | Description                                    |
| ------ | ------------- | ---------------------------------------------- |
| `data` | `map(secret)` | Data from the secret obtained from Kubernetes. |

The `data` field contains a mapping from field names to values.

If an individual key stored in `data` doesn't hold sensitive data, it can be converted into a string using [the `convert.nonsensitive` function][convert]:

```alloy
convert.nonsensitive(remote.kubernetes.secret.LABEL.data.KEY_NAME)
```

Using `convert.nonsensitive` allows for using the exports of `remote.kubernetes.secret` for attributes in components that don't support secrets.

[convert]: ../../../stdlib/convert/

## Component health

Instances of `remote.kubernetes.secret` report as healthy if the most recent attempt to poll the kubernetes API succeeds.

## Debug information

`remote.kubernetes.secret` doesn't expose any component-specific debug information.

## Debug metrics

`remote.kubernetes.secret` doesn't expose any component-specific debug metrics.

## Example

This example reads a Secret and a ConfigMap from Kubernetes and uses them to supply remote-write credentials.

```alloy
remote.kubernetes.secret "credentials" {
  namespace = "monitoring"
  name = "metrics-secret"
}

remote.kubernetes.configmap "endpoint" {
  namespace = "monitoring"
  name = "metrics-endpoint"
}

prometheus.remote_write "default" {
  endpoint {
    url = remote.kubernetes.configmap.endpoint.data["url"]
    basic_auth {
      username = convert.nonsensitive(remote.kubernetes.configmap.endpoint.data["username"])
      password = remote.kubernetes.secret.credentials.data["password"]
    }
  }
}
```

This example assumes that the Secret and ConfigMap have already been created, and that the appropriate field names exist in their data.
