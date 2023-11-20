/* eslint-disable */
// quota-control v0.1.0 38f778d4e99379e16caefb2109523359b47a04ee
// --
// Code generated by webrpc-gen@v0.13.0-dev with typescript@v0.12.0 generator. DO NOT EDIT.
//
// webrpc-gen -schema=proto.ridl -target=typescript@v0.12.0 -client -out=./clients/builder.gen.ts

// WebRPC description and code-gen version
export const WebRPCVersion = "v1"

// Schema version of your RIDL schema
export const WebRPCSchemaVersion = "v0.1.0"

// Schema hash generated from your RIDL schema
export const WebRPCSchemaHash = "38f778d4e99379e16caefb2109523359b47a04ee"

//
// Types
//


export enum Service {
  NodeGateway = 'NodeGateway',
  API = 'API',
  Indexer = 'Indexer',
  Relayer = 'Relayer',
  Metadata = 'Metadata'
}

export enum EventType {
  FreeCU = 'FreeCU',
  SoftQuota = 'SoftQuota',
  HardQuota = 'HardQuota'
}

export enum UserPermission {
  UNAUTHORIZED = 'UNAUTHORIZED',
  READ = 'READ',
  READ_WRITE = 'READ_WRITE',
  ADMIN = 'ADMIN'
}

export interface Limit {
  maxKeys: number
  rateLimit: number
  freeCU: number
  softQuota: number
  hardQuota: number
}

export interface AccessKey {
  projectId: number
  displayName: string
  accessKey: string
  active: boolean
  default: boolean
  allowedOrigins: Array<string>
  allowedServices: Array<Service>
  createdAt?: string
}

export interface AccessUsage {
  validCompute: number
  overCompute: number
  limitedCompute: number
}

export interface Cycle {
  start: string
  end: string
}

export interface AccessQuota {
  cycle: Cycle
  limit: Limit
  accessKey: AccessKey
}

export interface QuotaControl {
  getAccessLimit(args: GetAccessLimitArgs, headers?: object, signal?: AbortSignal): Promise<GetAccessLimitReturn>
  setAccessLimit(args: SetAccessLimitArgs, headers?: object, signal?: AbortSignal): Promise<SetAccessLimitReturn>
  getAccessKey(args: GetAccessKeyArgs, headers?: object, signal?: AbortSignal): Promise<GetAccessKeyReturn>
  getDefaultAccessKey(args: GetDefaultAccessKeyArgs, headers?: object, signal?: AbortSignal): Promise<GetDefaultAccessKeyReturn>
  createAccessKey(args: CreateAccessKeyArgs, headers?: object, signal?: AbortSignal): Promise<CreateAccessKeyReturn>
  rotateAccessKey(args: RotateAccessKeyArgs, headers?: object, signal?: AbortSignal): Promise<RotateAccessKeyReturn>
  updateAccessKey(args: UpdateAccessKeyArgs, headers?: object, signal?: AbortSignal): Promise<UpdateAccessKeyReturn>
  updateDefaultAccessKey(args: UpdateDefaultAccessKeyArgs, headers?: object, signal?: AbortSignal): Promise<UpdateDefaultAccessKeyReturn>
  listAccessKeys(args: ListAccessKeysArgs, headers?: object, signal?: AbortSignal): Promise<ListAccessKeysReturn>
  disableAccessKey(args: DisableAccessKeyArgs, headers?: object, signal?: AbortSignal): Promise<DisableAccessKeyReturn>
  getAccessQuota(args: GetAccessQuotaArgs, headers?: object, signal?: AbortSignal): Promise<GetAccessQuotaReturn>
  getAccountUsage(args: GetAccountUsageArgs, headers?: object, signal?: AbortSignal): Promise<GetAccountUsageReturn>
  getAccessKeyUsage(args: GetAccessKeyUsageArgs, headers?: object, signal?: AbortSignal): Promise<GetAccessKeyUsageReturn>
  prepareUsage(args: PrepareUsageArgs, headers?: object, signal?: AbortSignal): Promise<PrepareUsageReturn>
  notifyEvent(args: NotifyEventArgs, headers?: object, signal?: AbortSignal): Promise<NotifyEventReturn>
  updateUsage(args: UpdateUsageArgs, headers?: object, signal?: AbortSignal): Promise<UpdateUsageReturn>
  getUserPermission(args: GetUserPermissionArgs, headers?: object, signal?: AbortSignal): Promise<GetUserPermissionReturn>
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
export interface GetAccessKeyArgs {
  accessKey: string
}

export interface GetAccessKeyReturn {
  accessKey: AccessKey  
}
export interface GetDefaultAccessKeyArgs {
  projectID: number
}

export interface GetDefaultAccessKeyReturn {
  accessKey: AccessKey  
}
export interface CreateAccessKeyArgs {
  projectId: number
  displayName: string
  allowedOrigins: Array<string>
  allowedServices: Array<Service>
}

export interface CreateAccessKeyReturn {
  accessKey: AccessKey  
}
export interface RotateAccessKeyArgs {
  accessKey: string
}

export interface RotateAccessKeyReturn {
  accessKey: AccessKey  
}
export interface UpdateAccessKeyArgs {
  accessKey: string
  displayName?: string
  allowedOrigins?: Array<string>
  allowedServices?: Array<Service>
}

export interface UpdateAccessKeyReturn {
  accessKey: AccessKey  
}
export interface UpdateDefaultAccessKeyArgs {
  projectID: number
  accessKey: string
}

export interface UpdateDefaultAccessKeyReturn {
  ok: boolean  
}
export interface ListAccessKeysArgs {
  projectId: number
  active?: boolean
  service?: Service
}

export interface ListAccessKeysReturn {
  accessKeys: Array<AccessKey>  
}
export interface DisableAccessKeyArgs {
  accessKey: string
}

export interface DisableAccessKeyReturn {
  ok: boolean  
}
export interface GetAccessQuotaArgs {
  accessKey: string
}

export interface GetAccessQuotaReturn {
  accessQuota: AccessQuota  
}
export interface GetAccountUsageArgs {
  projectID: number
  service?: Service
  from?: string
  to?: string
}

export interface GetAccountUsageReturn {
  usage: AccessUsage  
}
export interface GetAccessKeyUsageArgs {
  accessKey: string
  service?: Service
  from?: string
  to?: string
}

export interface GetAccessKeyUsageReturn {
  usage: AccessUsage  
}
export interface PrepareUsageArgs {
  projectID: number
  now: string
}

export interface PrepareUsageReturn {
  ok: boolean  
}
export interface NotifyEventArgs {
  projectID: number
  eventType: EventType
}

export interface NotifyEventReturn {
  ok: boolean  
}
export interface UpdateUsageArgs {
  service: Service
  now: string
  usage: {[key: string]: AccessUsage}
}

export interface UpdateUsageReturn {
  ok: {[key: string]: boolean}  
}
export interface GetUserPermissionArgs {
  projectId: number
  userId: string
}

export interface GetUserPermissionReturn {
  permission: UserPermission
  resourceAccess: {[key: string]: any}  
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
  
  getAccessKey = (args: GetAccessKeyArgs, headers?: object, signal?: AbortSignal): Promise<GetAccessKeyReturn> => {
    return this.fetch(
      this.url('GetAccessKey'),
      createHTTPRequest(args, headers, signal)).then((res) => {
      return buildResponse(res).then(_data => {
        return {
          accessKey: <AccessKey>(_data.accessKey),
        }
      })
    }, (error) => {
      throw WebrpcRequestFailedError.new({ cause: `fetch(): ${error.message || ''}` })
    })
  }
  
  getDefaultAccessKey = (args: GetDefaultAccessKeyArgs, headers?: object, signal?: AbortSignal): Promise<GetDefaultAccessKeyReturn> => {
    return this.fetch(
      this.url('GetDefaultAccessKey'),
      createHTTPRequest(args, headers, signal)).then((res) => {
      return buildResponse(res).then(_data => {
        return {
          accessKey: <AccessKey>(_data.accessKey),
        }
      })
    }, (error) => {
      throw WebrpcRequestFailedError.new({ cause: `fetch(): ${error.message || ''}` })
    })
  }
  
  createAccessKey = (args: CreateAccessKeyArgs, headers?: object, signal?: AbortSignal): Promise<CreateAccessKeyReturn> => {
    return this.fetch(
      this.url('CreateAccessKey'),
      createHTTPRequest(args, headers, signal)).then((res) => {
      return buildResponse(res).then(_data => {
        return {
          accessKey: <AccessKey>(_data.accessKey),
        }
      })
    }, (error) => {
      throw WebrpcRequestFailedError.new({ cause: `fetch(): ${error.message || ''}` })
    })
  }
  
  rotateAccessKey = (args: RotateAccessKeyArgs, headers?: object, signal?: AbortSignal): Promise<RotateAccessKeyReturn> => {
    return this.fetch(
      this.url('RotateAccessKey'),
      createHTTPRequest(args, headers, signal)).then((res) => {
      return buildResponse(res).then(_data => {
        return {
          accessKey: <AccessKey>(_data.accessKey),
        }
      })
    }, (error) => {
      throw WebrpcRequestFailedError.new({ cause: `fetch(): ${error.message || ''}` })
    })
  }
  
  updateAccessKey = (args: UpdateAccessKeyArgs, headers?: object, signal?: AbortSignal): Promise<UpdateAccessKeyReturn> => {
    return this.fetch(
      this.url('UpdateAccessKey'),
      createHTTPRequest(args, headers, signal)).then((res) => {
      return buildResponse(res).then(_data => {
        return {
          accessKey: <AccessKey>(_data.accessKey),
        }
      })
    }, (error) => {
      throw WebrpcRequestFailedError.new({ cause: `fetch(): ${error.message || ''}` })
    })
  }
  
  updateDefaultAccessKey = (args: UpdateDefaultAccessKeyArgs, headers?: object, signal?: AbortSignal): Promise<UpdateDefaultAccessKeyReturn> => {
    return this.fetch(
      this.url('UpdateDefaultAccessKey'),
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
  
  listAccessKeys = (args: ListAccessKeysArgs, headers?: object, signal?: AbortSignal): Promise<ListAccessKeysReturn> => {
    return this.fetch(
      this.url('ListAccessKeys'),
      createHTTPRequest(args, headers, signal)).then((res) => {
      return buildResponse(res).then(_data => {
        return {
          accessKeys: <Array<AccessKey>>(_data.accessKeys),
        }
      })
    }, (error) => {
      throw WebrpcRequestFailedError.new({ cause: `fetch(): ${error.message || ''}` })
    })
  }
  
  disableAccessKey = (args: DisableAccessKeyArgs, headers?: object, signal?: AbortSignal): Promise<DisableAccessKeyReturn> => {
    return this.fetch(
      this.url('DisableAccessKey'),
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
  
  getAccessQuota = (args: GetAccessQuotaArgs, headers?: object, signal?: AbortSignal): Promise<GetAccessQuotaReturn> => {
    return this.fetch(
      this.url('GetAccessQuota'),
      createHTTPRequest(args, headers, signal)).then((res) => {
      return buildResponse(res).then(_data => {
        return {
          accessQuota: <AccessQuota>(_data.accessQuota),
        }
      })
    }, (error) => {
      throw WebrpcRequestFailedError.new({ cause: `fetch(): ${error.message || ''}` })
    })
  }
  
  getAccountUsage = (args: GetAccountUsageArgs, headers?: object, signal?: AbortSignal): Promise<GetAccountUsageReturn> => {
    return this.fetch(
      this.url('GetAccountUsage'),
      createHTTPRequest(args, headers, signal)).then((res) => {
      return buildResponse(res).then(_data => {
        return {
          usage: <AccessUsage>(_data.usage),
        }
      })
    }, (error) => {
      throw WebrpcRequestFailedError.new({ cause: `fetch(): ${error.message || ''}` })
    })
  }
  
  getAccessKeyUsage = (args: GetAccessKeyUsageArgs, headers?: object, signal?: AbortSignal): Promise<GetAccessKeyUsageReturn> => {
    return this.fetch(
      this.url('GetAccessKeyUsage'),
      createHTTPRequest(args, headers, signal)).then((res) => {
      return buildResponse(res).then(_data => {
        return {
          usage: <AccessUsage>(_data.usage),
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
  
  notifyEvent = (args: NotifyEventArgs, headers?: object, signal?: AbortSignal): Promise<NotifyEventReturn> => {
    return this.fetch(
      this.url('NotifyEvent'),
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
  
  getUserPermission = (args: GetUserPermissionArgs, headers?: object, signal?: AbortSignal): Promise<GetUserPermissionReturn> => {
    return this.fetch(
      this.url('GetUserPermission'),
      createHTTPRequest(args, headers, signal)).then((res) => {
      return buildResponse(res).then(_data => {
        return {
          permission: <UserPermission>(_data.permission),
          resourceAccess: <{[key: string]: any}>(_data.resourceAccess),
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

export class WebrpcInternalErrorError extends WebrpcError {
  constructor(
    name: string = 'WebrpcInternalError',
    code: number = -7,
    message: string = 'internal error',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, WebrpcInternalErrorError.prototype)
  }
}


// Schema errors

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

export class AccessKeyNotFoundError extends WebrpcError {
  constructor(
    name: string = 'AccessKeyNotFound',
    code: number = 1005,
    message: string = 'Access key not found',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, AccessKeyNotFoundError.prototype)
  }
}

export class UnauthorizedUserError extends WebrpcError {
  constructor(
    name: string = 'UnauthorizedUser',
    code: number = 1006,
    message: string = 'Unauthorized user',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, UnauthorizedUserError.prototype)
  }
}

export class MaxAccessKeysError extends WebrpcError {
  constructor(
    name: string = 'MaxAccessKeys',
    code: number = 1007,
    message: string = 'Access keys limit reached',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, MaxAccessKeysError.prototype)
  }
}

export class NoDefaultKeyError extends WebrpcError {
  constructor(
    name: string = 'NoDefaultKey',
    code: number = 1008,
    message: string = 'No default access key found',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, NoDefaultKeyError.prototype)
  }
}

export class AtLeastOneKeyError extends WebrpcError {
  constructor(
    name: string = 'AtLeastOneKey',
    code: number = 1009,
    message: string = 'There should be at least one active accessKey per project',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, AtLeastOneKeyError.prototype)
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
  WebrpcInternalError = 'WebrpcInternalError',
  Timeout = 'Timeout',
  LimitExceeded = 'LimitExceeded',
  InvalidOrigin = 'InvalidOrigin',
  InvalidService = 'InvalidService',
  AccessKeyNotFound = 'AccessKeyNotFound',
  UnauthorizedUser = 'UnauthorizedUser',
  MaxAccessKeys = 'MaxAccessKeys',
  NoDefaultKey = 'NoDefaultKey',
  AtLeastOneKey = 'AtLeastOneKey',
}

const webrpcErrorByCode: { [code: number]: any } = {
  [0]: WebrpcEndpointError,
  [-1]: WebrpcRequestFailedError,
  [-2]: WebrpcBadRouteError,
  [-3]: WebrpcBadMethodError,
  [-4]: WebrpcBadRequestError,
  [-5]: WebrpcBadResponseError,
  [-6]: WebrpcServerPanicError,
  [-7]: WebrpcInternalErrorError,
  [1001]: TimeoutError,
  [1002]: LimitExceededError,
  [1003]: InvalidOriginError,
  [1004]: InvalidServiceError,
  [1005]: AccessKeyNotFoundError,
  [1006]: UnauthorizedUserError,
  [1007]: MaxAccessKeysError,
  [1008]: NoDefaultKeyError,
  [1009]: AtLeastOneKeyError,
}

export type Fetch = (input: RequestInfo, init?: RequestInit) => Promise<Response>
