# Authentication Token Utility

The `tesla-auth-token` utility reads an OAuth token from `stdin` or a
designated file and writes the token to the system keyring.

This tool does **not** obtain Tesla partner tokens
(`grant_type=client_credentials`). Tesla Fleet Auth `invalid_audience`
and `/authorize` "No policy rules" are Tesla Identity Provider
provisioning
([issue #460](https://github.com/teslamotors/vehicle-command/issues/460)).
`tesla-control partner-oauth` returns
`protocol.ErrPartnerOAuthNotProvisioned`. Use Tesla developer dashboard
Support Inquiry; do not send client secrets to this repository.

The mechanism used for the keyring is OS-specific, and can be configured using
command-line flags or the environment. Run `tesla-auth-token -h` for more
information.
