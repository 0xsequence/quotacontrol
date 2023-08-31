/* eslint-disable */
// quota-control v0.1.0 3f9ec3592f457fc001584878d0728650bef23746
// --
// Code generated by webrpc-gen@v0.12.x-dev with typescript@v0.12.0 generator. DO NOT EDIT.
//
// webrpc-gen -schema=proto.ridl -target=typescript@v0.12.0 -client -out=./clients/builder.gen.ts

// WebRPC description and code-gen version
export const WebRPCVersion = "v1"

// Schema version of your RIDL schema
export const WebRPCSchemaVersion = "v0.1.0"

// Schema hash generated from your RIDL schema
export const WebRPCSchemaHash = "3f9ec3592f457fc001584878d0728650bef23746"

//
// Types
//


export enum Service {
  NodeGateway = 'NodeGateway',
  Indexer = 'Indexer'
}

export interface Limit {
  rateLimit: number
  freeCU: number
  softQuota: number
  hardQuota: number
}

export interface AccessToken {
  projectId: number
  displayName: string
  tokenKey: string
  active: boolean
  allowedOrigins: Array<string>
  allowedServices: Array<Service>
  createdAt?: string
}

export interface AccessTokenUsage {
  validCompute: number
  overCompute: number
  limitedCompute: number
}

export interface CachedToken {
  limit: Limit
  accessToken: AccessToken
}

export interface QuotaControl {
  getAccessLimit(args: GetAccessLimitArgs, headers?: object, signal?: AbortSignal): Promise<GetAccessLimitReturn>
  setAccessLimit(args: SetAccessLimitArgs, headers?: object, signal?: AbortSignal): Promise<SetAccessLimitReturn>
  getAccessToken(args: GetAccessTokenArgs, headers?: object, signal?: AbortSignal): Promise<GetAccessTokenReturn>
  createAccessToken(args: CreateAccessTokenArgs, headers?: object, signal?: AbortSignal): Promise<CreateAccessTokenReturn>
  updateAccessToken(args: UpdateAccessTokenArgs, headers?: object, signal?: AbortSignal): Promise<UpdateAccessTokenReturn>
  listAccessTokens(args: ListAccessTokensArgs, headers?: object, signal?: AbortSignal): Promise<ListAccessTokensReturn>
  disableAccessToken(args: DisableAccessTokenArgs, headers?: object, signal?: AbortSignal): Promise<DisableAccessTokenReturn>
  getUsage(args: GetUsageArgs, headers?: object, signal?: AbortSignal): Promise<GetUsageReturn>
  prepareUsage(args: PrepareUsageArgs, headers?: object, signal?: AbortSignal): Promise<PrepareUsageReturn>
  retrieveToken(args: RetrieveTokenArgs, headers?: object, signal?: AbortSignal): Promise<RetrieveTokenReturn>
  updateUsage(args: UpdateUsageArgs, headers?: object, signal?: AbortSignal): Promise<UpdateUsageReturn>
}

export interface GetAccessLimitArgs {
  projectId: number
}

export interface GetAccessLimitReturn {
  limit: Limit  
}
export interface SetAccessLimitArgs {
  projectId: number
  limit: Limit
}

export interface SetAccessLimitReturn {
  ok: boolean  
}
export interface GetAccessTokenArgs {
  tokenKey: string
}

export interface GetAccessTokenReturn {
  accessToken: AccessToken  
}
export interface CreateAccessTokenArgs {
  projectId: number
  displayName: string
  allowedOrigins: Array<string>
  allowedServices: Array<Service>
}

export interface CreateAccessTokenReturn {
  accessToken: AccessToken  
}
export interface UpdateAccessTokenArgs {
  tokenKey: string
  displayName?: string
  allowedOrigins?: Array<string>
  allowedServices?: Array<Service>
}

export interface UpdateAccessTokenReturn {
  accessToken: AccessToken  
}
export interface ListAccessTokensArgs {
  projectId: number
}

export interface ListAccessTokensReturn {
  accessTokens: Array<AccessToken>  
}
export interface DisableAccessTokenArgs {
  tokenKey: string
}

export interface DisableAccessTokenReturn {
  ok: boolean  
}
export interface GetUsageArgs {
  projectID: number
  service?: Service
  now: string
}

export interface GetUsageReturn {
  usage: AccessTokenUsage  
}
export interface PrepareUsageArgs {
  projectID: number
  now: string
}

export interface PrepareUsageReturn {
  ok: boolean  
}
export interface RetrieveTokenArgs {
  tokenKey: string
}

export interface RetrieveTokenReturn {
  token: CachedToken  
}
export interface UpdateUsageArgs {
  service: Service
  now: string
  usage: {[key: string]: AccessTokenUsage}
}

export interface UpdateUsageReturn {
  ok: {[key: string]: boolean}  
}


  
//
// Client
//
export class QuotaControl implements QuotaControl {
  protected hostname: string
  protected fetch: Fetch
  protected path = '/rpc/QuotaControl/'

  constructor(hostname: string, fetch: Fetch) {
    this.hostname = hostname
    this.fetch = (input: RequestInfo, init?: RequestInit) => fetch(input, init)
  }

  private url(name: string): string {
    return this.hostname + this.path + name
  }
  
  getAccessLimit = (args: GetAccessLimitArgs, headers?: object, signal?: AbortSignal): Promise<GetAccessLimitReturn> => {
    return this.fetch(
      this.url('GetAccessLimit'),
      createHTTPRequest(args, headers, signal)).then((res) => {
      return buildResponse(res).then(_data => {
        return {
          limit: <Limit>(_data.limit),
        }
      })
    }, (error) => {
      throw WebrpcRequestFailedError.new({ cause: `fetch(): ${error.message || ''}` })
    })
  }
  
  setAccessLimit = (args: SetAccessLimitArgs, headers?: object, signal?: AbortSignal): Promise<SetAccessLimitReturn> => {
    return this.fetch(
      this.url('SetAccessLimit'),
      createHTTPRequest(args, headers, signal)).then((res) => {
      return buildResponse(res).then(_data => {
        return {
          ok: <boolean>(_data.ok),
        }
      })
    }, (error) => {
      throw WebrpcRequestFailedError.new({ cause: `fetch(): ${error.message || ''}` })
    })
  }
  
  getAccessToken = (args: GetAccessTokenArgs, headers?: object, signal?: AbortSignal): Promise<GetAccessTokenReturn> => {
    return this.fetch(
      this.url('GetAccessToken'),
      createHTTPRequest(args, headers, signal)).then((res) => {
      return buildResponse(res).then(_data => {
        return {
          accessToken: <AccessToken>(_data.accessToken),
        }
      })
    }, (error) => {
      throw WebrpcRequestFailedError.new({ cause: `fetch(): ${error.message || ''}` })
    })
  }
  
  createAccessToken = (args: CreateAccessTokenArgs, headers?: object, signal?: AbortSignal): Promise<CreateAccessTokenReturn> => {
    return this.fetch(
      this.url('CreateAccessToken'),
      createHTTPRequest(args, headers, signal)).then((res) => {
      return buildResponse(res).then(_data => {
        return {
          accessToken: <AccessToken>(_data.accessToken),
        }
      })
    }, (error) => {
      throw WebrpcRequestFailedError.new({ cause: `fetch(): ${error.message || ''}` })
    })
  }
  
  updateAccessToken = (args: UpdateAccessTokenArgs, headers?: object, signal?: AbortSignal): Promise<UpdateAccessTokenReturn> => {
    return this.fetch(
      this.url('UpdateAccessToken'),
      createHTTPRequest(args, headers, signal)).then((res) => {
      return buildResponse(res).then(_data => {
        return {
          accessToken: <AccessToken>(_data.accessToken),
        }
      })
    }, (error) => {
      throw WebrpcRequestFailedError.new({ cause: `fetch(): ${error.message || ''}` })
    })
  }
  
  listAccessTokens = (args: ListAccessTokensArgs, headers?: object, signal?: AbortSignal): Promise<ListAccessTokensReturn> => {
    return this.fetch(
      this.url('ListAccessTokens'),
      createHTTPRequest(args, headers, signal)).then((res) => {
      return buildResponse(res).then(_data => {
        return {
          accessTokens: <Array<AccessToken>>(_data.accessTokens),
        }
      })
    }, (error) => {
      throw WebrpcRequestFailedError.new({ cause: `fetch(): ${error.message || ''}` })
    })
  }
  
  disableAccessToken = (args: DisableAccessTokenArgs, headers?: object, signal?: AbortSignal): Promise<DisableAccessTokenReturn> => {
    return this.fetch(
      this.url('DisableAccessToken'),
      createHTTPRequest(args, headers, signal)).then((res) => {
      return buildResponse(res).then(_data => {
        return {
          ok: <boolean>(_data.ok),
        }
      })
    }, (error) => {
      throw WebrpcRequestFailedError.new({ cause: `fetch(): ${error.message || ''}` })
    })
  }
  
  getUsage = (args: GetUsageArgs, headers?: object, signal?: AbortSignal): Promise<GetUsageReturn> => {
    return this.fetch(
      this.url('GetUsage'),
      createHTTPRequest(args, headers, signal)).then((res) => {
      return buildResponse(res).then(_data => {
        return {
          usage: <AccessTokenUsage>(_data.usage),
        }
      })
    }, (error) => {
      throw WebrpcRequestFailedError.new({ cause: `fetch(): ${error.message || ''}` })
    })
  }
  
  prepareUsage = (args: PrepareUsageArgs, headers?: object, signal?: AbortSignal): Promise<PrepareUsageReturn> => {
    return this.fetch(
      this.url('PrepareUsage'),
      createHTTPRequest(args, headers, signal)).then((res) => {
      return buildResponse(res).then(_data => {
        return {
          ok: <boolean>(_data.ok),
        }
      })
    }, (error) => {
      throw WebrpcRequestFailedError.new({ cause: `fetch(): ${error.message || ''}` })
    })
  }
  
  retrieveToken = (args: RetrieveTokenArgs, headers?: object, signal?: AbortSignal): Promise<RetrieveTokenReturn> => {
    return this.fetch(
      this.url('RetrieveToken'),
      createHTTPRequest(args, headers, signal)).then((res) => {
      return buildResponse(res).then(_data => {
        return {
          token: <CachedToken>(_data.token),
        }
      })
    }, (error) => {
      throw WebrpcRequestFailedError.new({ cause: `fetch(): ${error.message || ''}` })
    })
  }
  
  updateUsage = (args: UpdateUsageArgs, headers?: object, signal?: AbortSignal): Promise<UpdateUsageReturn> => {
    return this.fetch(
      this.url('UpdateUsage'),
      createHTTPRequest(args, headers, signal)).then((res) => {
      return buildResponse(res).then(_data => {
        return {
          ok: <{[key: string]: boolean}>(_data.ok),
        }
      })
    }, (error) => {
      throw WebrpcRequestFailedError.new({ cause: `fetch(): ${error.message || ''}` })
    })
  }
  
}

  const createHTTPRequest = (body: object = {}, headers: object = {}, signal: AbortSignal | null = null): object => {
  return {
    method: 'POST',
    headers: { ...headers, 'Content-Type': 'application/json' },
    body: JSON.stringify(body || {}),
    signal
  }
}

const buildResponse = (res: Response): Promise<any> => {
  return res.text().then(text => {
    let data
    try {
      data = JSON.parse(text)
    } catch(error) {
      let message = ''
      if (error instanceof Error)  {
        message = error.message
      }
      throw WebrpcBadResponseError.new({
        status: res.status,
        cause: `JSON.parse(): ${message}: response text: ${text}`},
      )
    }
    if (!res.ok) {
      const code: number = (typeof data.code === 'number') ? data.code : 0
      throw (webrpcErrorByCode[code] || WebrpcError).new(data)
    }
    return data
  })
}

//
// Errors
//

export class WebrpcError extends Error {
  name: string
  code: number
  message: string
  status: number
  cause?: string

  /** @deprecated Use message instead of msg. Deprecated in webrpc v0.11.0. */
  msg: string

  constructor(name: string, code: number, message: string, status: number, cause?: string) {
    super(message)
    this.name = name || 'WebrpcError'
    this.code = typeof code === 'number' ? code : 0
    this.message = message || `endpoint error ${this.code}`
    this.msg = this.message
    this.status = typeof status === 'number' ? status : 0
    this.cause = cause
    Object.setPrototypeOf(this, WebrpcError.prototype)
  }

  static new(payload: any): WebrpcError {
    return new this(payload.error, payload.code, payload.message || payload.msg, payload.status, payload.cause)
  }
}

// Webrpc errors

export class WebrpcEndpointError extends WebrpcError {
  constructor(
    name: string = 'WebrpcEndpoint',
    code: number = 0,
    message: string = 'endpoint error',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, WebrpcEndpointError.prototype)
  }
}

export class WebrpcRequestFailedError extends WebrpcError {
  constructor(
    name: string = 'WebrpcRequestFailed',
    code: number = -1,
    message: string = 'request failed',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, WebrpcRequestFailedError.prototype)
  }
}

export class WebrpcBadRouteError extends WebrpcError {
  constructor(
    name: string = 'WebrpcBadRoute',
    code: number = -2,
    message: string = 'bad route',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, WebrpcBadRouteError.prototype)
  }
}

export class WebrpcBadMethodError extends WebrpcError {
  constructor(
    name: string = 'WebrpcBadMethod',
    code: number = -3,
    message: string = 'bad method',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, WebrpcBadMethodError.prototype)
  }
}

export class WebrpcBadRequestError extends WebrpcError {
  constructor(
    name: string = 'WebrpcBadRequest',
    code: number = -4,
    message: string = 'bad request',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, WebrpcBadRequestError.prototype)
  }
}

export class WebrpcBadResponseError extends WebrpcError {
  constructor(
    name: string = 'WebrpcBadResponse',
    code: number = -5,
    message: string = 'bad response',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, WebrpcBadResponseError.prototype)
  }
}

export class WebrpcServerPanicError extends WebrpcError {
  constructor(
    name: string = 'WebrpcServerPanic',
    code: number = -6,
    message: string = 'server panic',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, WebrpcServerPanicError.prototype)
  }
}


// Schema errors

export class NotImplementedError extends WebrpcError {
  constructor(
    name: string = 'NotImplemented',
    code: number = 1000,
    message: string = 'Endpoint not implemented',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, NotImplementedError.prototype)
  }
}

export class TimeoutError extends WebrpcError {
  constructor(
    name: string = 'Timeout',
    code: number = 1001,
    message: string = 'Request timed out',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, TimeoutError.prototype)
  }
}

export class LimitExceededError extends WebrpcError {
  constructor(
    name: string = 'LimitExceeded',
    code: number = 1002,
    message: string = 'Request limit exceeded',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, LimitExceededError.prototype)
  }
}

export class InvalidOriginError extends WebrpcError {
  constructor(
    name: string = 'InvalidOrigin',
    code: number = 1003,
    message: string = 'Invalid origin',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, InvalidOriginError.prototype)
  }
}

export class InvalidServiceError extends WebrpcError {
  constructor(
    name: string = 'InvalidService',
    code: number = 1004,
    message: string = 'Invalid service',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, InvalidServiceError.prototype)
  }
}

export class TokenNotFoundError extends WebrpcError {
  constructor(
    name: string = 'TokenNotFound',
    code: number = 1005,
    message: string = 'Token not found',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, TokenNotFoundError.prototype)
  }
}


export enum errors {
  WebrpcEndpoint = 'WebrpcEndpoint',
  WebrpcRequestFailed = 'WebrpcRequestFailed',
  WebrpcBadRoute = 'WebrpcBadRoute',
  WebrpcBadMethod = 'WebrpcBadMethod',
  WebrpcBadRequest = 'WebrpcBadRequest',
  WebrpcBadResponse = 'WebrpcBadResponse',
  WebrpcServerPanic = 'WebrpcServerPanic',
  NotImplemented = 'NotImplemented',
  Timeout = 'Timeout',
  LimitExceeded = 'LimitExceeded',
  InvalidOrigin = 'InvalidOrigin',
  InvalidService = 'InvalidService',
  TokenNotFound = 'TokenNotFound',
}

const webrpcErrorByCode: { [code: number]: any } = {
  [0]: WebrpcEndpointError,
  [-1]: WebrpcRequestFailedError,
  [-2]: WebrpcBadRouteError,
  [-3]: WebrpcBadMethodError,
  [-4]: WebrpcBadRequestError,
  [-5]: WebrpcBadResponseError,
  [-6]: WebrpcServerPanicError,
  [1000]: NotImplementedError,
  [1001]: TimeoutError,
  [1002]: LimitExceededError,
  [1003]: InvalidOriginError,
  [1004]: InvalidServiceError,
  [1005]: TokenNotFoundError,
}

export type Fetch = (input: RequestInfo, init?: RequestInit) => Promise<Response>
