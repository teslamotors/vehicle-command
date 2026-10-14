/*
Tesla-auth-token writes a provided OAuth token to the system keyring.

It does not mint Tesla partner tokens. Tesla Fleet Auth invalid_audience
and /authorize "No policy rules" are Tesla Identity Provider
provisioning (teslamotors/vehicle-command#460).
*/
package main
