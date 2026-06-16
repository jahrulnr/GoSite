# Artifact signing

Production installs require a trusted Ed25519 signature unless
`PLUGIN_ALLOW_UNSIGNED=true` (dev only).

## Generate a keypair

```bash
go run ../../_shared/scripts/keygen.go -out ~/.config/gosite/signing
# writes signing.key (private) and signing.pub.json (public key record)
```

Register the public key with the host:

```http
POST /api/v1/plugins/keyring
{
  "vendor": "acme",
  "keyId": "acme-1",
  "publicKey": "<base64 raw 32-byte public key>"
}
```

## Sign a zip

```bash
make sign KEY=~/.config/gosite/signing.key KEY_ID=acme-1
```

This updates `manifest.json` inside the zip with `signatures[]` and writes
`<artifact>.zip.sigmeta` recording:

- `signedDigest` — digest that was signed (pre-embed)
- `uploadDigest` — digest of the final zip bytes (use for `sha256=` on install)

The host verifies Ed25519 signatures over the **uploaded** zip digest. Because
embedding signatures changes the zip bytes, use `PLUGIN_ALLOW_UNSIGNED=true` for
local dev or register the key and upload with `uploadDigest` from sigmeta.

## Verify locally

```bash
go run ../../_shared/scripts/verify.go -artifact dist/my-plugin.zip -pub signing.pub.json
```
