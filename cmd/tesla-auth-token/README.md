# Authentication Token Utility

The `tesla-auth-token` utility reads an OAuth token from `stdin` or a
designated file and writes the token to the system keyring.

The mechanism used for the keyring is OS-specific, and can be configured using
command-line flags or the environment. Run `tesla-auth-token -h` for more
information.
