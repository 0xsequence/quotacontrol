# Quota Control
![Go Workflow](https://github.com/0xsequence/quotacontrol/actions/workflows/go.yml/badge.svg)
[![Go Reference](https://pkg.go.dev/badge/github.com/0xsequence/quotacontrol.svg)](https://pkg.go.dev/github.com/0xsequence/quotacontrol)

This package implements a Quota Control - Rate Limiting service. 
It allows to limit the amount of compute units spent by projects.

# Entities
- AccessKey: a specific key used for Quota Control, it belongs to a project.
- Limit: attribute of project, a series of limitations like quota and number of keys.
- AccessUsage: the usage of certain project or key. It has 3 values:
  - valid: requests done using the "Free Limit" limit
  - over: requests over "Free Limit" and below "Hard Limit"
  - limited: request over "Hard Limit" which result in a failure (HTTP 429)
- Cycle:a range of time where limits should be enforced (e.g. monthly, weekly)  

# Actors
There are two actors at play:
- server: the main responsabilities are taking care of persisting records and preparing cache entries
- client: it communicates with cache and server to retrieve record. It does it using a series of [middlewares](https://github.com/0xsequence/quotacontrol/blob/062a68e96a4de99b85c38d4f4d6f66346311e961/middleware/).
  - `SetAccessKey`: looks into a specific header (`X-Access-Key`) to set the access key.
  - `VerifyAccessKey`: fetches the data related to the access key (timeframe, limits, access key metadata) and verifies that the provided key is valid.
  - `SpendUsage`: tries to increment the usage counter in the cache and compares it to the existing limits.

The actors share information through the cache, and server is the one populating it. 
The only execption is the usage spending, which is done by the clients using an increment operation on the cache.

# Configuration
The configuration for the service can be found here:
https://github.com/0xsequence/quotacontrol/blob/062a68e96a4de99b85c38d4f4d6f66346311e961/config.go#L19-L35

# Service

The `QuotaControlService` server requires two storages: a cache and a permanent store.

https://github.com/0xsequence/quotacontrol/blob/062a68e96a4de99b85c38d4f4d6f66346311e961/quotacontrol.go#L40-L53

Each aspect of the cache and the storage can be re-implemented and customised to the project needs.

The package offers a Redis implementation for the cache, and a Memory version of the permanent store useful for testing.



The methods that are used to save/load in a permanent storage the 3 entities are not implemented.
The requests are measure in compute units, if a compute unit is not specified it is assumed that the value it's 1.
A client can specify the amount of compute units by manipulating the request context using the `WithComputeUnits` function.

# Increment operation

The client method `SpendComputeUnits` takes care of doing an increment operation in the cache. And works as follows:
- It tries to fetch the usage record from the cache
- On a hit it executes the INCR operation.
- On a miss it sets it to `-1` and ask the server to populate it.
- If it finds the value `-1` it waits and retries
- After a few retries it fails with a timeout.
