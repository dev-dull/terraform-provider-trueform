---
page_title: "trueform_certificate Resource - Trueform"
subcategory: "System"
description: |-
  Manages an SSL/TLS certificate on TrueNAS.
---

# trueform_certificate (Resource)

Manages an SSL/TLS certificate on TrueNAS Scale. Supports internal certificate generation, imported certificates, and CSR creation.

~> **Note:** Changing the `name` or `type` will force recreation of the certificate. Most certificate fields cannot be updated after creation.

## Example Usage

### Import an Existing Certificate

```hcl
resource "trueform_certificate" "web" {
  name        = "web-cert"
  type        = "CERTIFICATE_CREATE_IMPORTED"
  certificate = file("cert.pem")
  privatekey  = file("key.pem")
}
```

### Create an Internal Certificate

```hcl
resource "trueform_certificate" "internal" {
  name             = "internal-cert"
  type             = "CERTIFICATE_CREATE_INTERNAL"
  key_length       = 2048
  digest_algorithm = "SHA256"
  lifetime         = 3650

  country      = "US"
  state        = "California"
  city         = "San Francisco"
  organization = "Example Inc"
  common_name  = "truenas.example.com"

  san = ["truenas.example.com", "192.168.1.100"]
}
```

## Schema

### Required

- `name` (String) The name of the certificate. Cannot be changed after creation.
- `type` (String) Certificate creation type. Values: `CERTIFICATE_CREATE_INTERNAL`, `CERTIFICATE_CREATE_IMPORTED`, `CERTIFICATE_CREATE_CSR`, `CERTIFICATE_CREATE_ACME`. Cannot be changed after creation.

### Optional

- `cert_chain` (Boolean) Include certificate chain.
- `certificate` (String, Sensitive) PEM-encoded certificate (for imported certificates).
- `city` (String) City or locality.
- `common_name` (String) Common name (CN).
- `country` (String) Country code.
- `digest_algorithm` (String) Digest algorithm. Values: `SHA256`, `SHA384`, `SHA512`.
- `email` (String) Email address.
- `key_length` (Number) RSA key length. Values: `1024`, `2048`, `4096`. Defaults to `2048`.
- `key_type` (String) Key type. Values: `RSA`, `EC`.
- `lifetime` (Number) Certificate lifetime in days. Defaults to `3650`.
- `organization` (String) Organization name.
- `organizational_unit` (String) Organizational unit.
- `privatekey` (String, Sensitive) PEM-encoded private key (for imported certificates).
- `san` (List of String) Subject Alternative Names.
- `signedby` (Number) ID of the CA that signed this certificate.
- `state` (String) State or province.

### Read-Only

- `csr` (String) Certificate Signing Request.
- `fingerprint` (String) Certificate fingerprint.
- `id` (Number) Certificate identifier.
- `not_after` (String) Certificate validity end date.
- `not_before` (String) Certificate validity start date.

## Import

Certificates can be imported using the certificate ID:

```shell
terraform import trueform_certificate.web 1
```
