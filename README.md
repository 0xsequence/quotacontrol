# Quota Control

This package implements a Quota Control - Rate Limiting service.

For each Dapp there are a series of `ServiceLimit`: it defines the monthly quotas and hour rate limit for each Service.
Dapps have a series of `AccessToken` that are used to execute calls that are limited by `ServiceLimit`.
The last entity is `AccessTokenUsage` which represent the usage of a token for a certain period.

# Service

The implementation of the `QuotaControlService` it's incomplete. The methods that are used to save/load in a permanent storage the 3 entities are not implemented.
The requests are measure in compute units, if a compute unit is not specified it is assumed that the value it's 1.
A client can specify the amount of compute units by manipulatig the request context using the `WithComputeUnits` function.

# Middleware
The clients will have to create a `QuotaControlClient` in order to use the provided middleware.
The client saves the `AccessTokenUsage` changes in memory. If the `QuotaControlClient.Run` method is called, 
they will periodically sent to the `QuotaControlService` which will store them periodically.

The clients that want to use the service can use the existing middleware which does the following:

- It tries to get the `AccessToken` from Cache, falling back to `QuotaControlService.RetrieveToken` 
(which also prepare Cache for future usage).
- Checks for `ServiceLimit` validity and for `AccessToken` to be active.
- Checks if the call origin matches one of the `AccessToken.AllowedOrigins`, if specified.
- Checks if the service matches one of the `AccessToken.AllowedServices`, if specified.
- Uses `QuotaControlService.SpendComputeUnits`.
- If `CACHE_PING` is returned it calls for `QuotaControlService.PrepareUsage` and tries again
- If `CACHE_WAIT_AND_RETRY` waits and tries again.
- If `CACHE_ALLOWED` it updates the number of `ValidCompute` and continues with the next http handler.
- If `CACHE_LIMITED` it updates the number of `LimitCompute` and stops.