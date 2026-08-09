// Package account implements functions for managing a Tesla account.
//
// For long-lived applications, construct an Account with [FromTokenSource]
// so OAuth access tokens can be refreshed automatically. [New] accepts a
// static access token and does not refresh it.
package account
