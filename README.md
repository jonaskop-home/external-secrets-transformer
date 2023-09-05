# External Secrets Transformer

Transform kubernetes secrets to external secrets automatically.

## Idea

I had an issue where I wanted to use third party helm charts (Argo CD, Prometheus, etc). But I had all my secrets in a keyvault. I didn't want to update or maintain all those charts by myself so therefore I made this tool to help me out with that.

## How it works

The program reads yaml documents from stdin and transforms the secrets which includes go template variables in their .data or .stringData fields where it converts the ones containing `{{ someVariable }}` to external secrets.

## Usage

To use it, you must set these environment variables:
| Name | Required | Default Value |
|------|----------|---------------|
| `STORE_NAME` | X | |
| `STORE_KIND` | X | |
| `REFRESH_INTERVAL` | | `1h` |

## Example

Example with `STORE_NAME=gcp-store` and ` STORE_KIND=ClusterSecretStore`

**Before**

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: dotfile-secret
data:
  apiToken: e3sgYXBpLXRva2VuIH19
stringData:
  config-file.json: |
    {
      "username": "admin123",
      "password": "{{ password }}",
    }
```

**Before**

```yaml
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: dotfile-secret
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: gcp-store
    kind: ClusterSecretStore
  target:
    template:
      data:
        apiToken: "{{ api-token }}"
        config-file.json: |
          {
            "username": "admin123",
            "password": "{{ password }}",
          }
  data:
    - secretKey: password
      remoteRef:
        key: password
    - secretKey: api-token
      remoteRef:
        key: api-token

---
```
