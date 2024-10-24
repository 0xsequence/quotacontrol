/* eslint-disable */
// quota-control v0.17.1 15948e7930b709422fbfdf66eb70f4e51a54b37b
// --
// Code generated by webrpc-gen@v0.21.0 with typescript@v0.12.0 generator. DO NOT EDIT.
//
// webrpc-gen -schema=proto.ridl -target=typescript@v0.12.0 -client -out=./clients/quotacontrol.gen.ts

// WebRPC description and code-gen version
export const WebRPCVersion = "v1"

// Schema version of your RIDL schema
export const WebRPCSchemaVersion = "v0.17.1"

// Schema hash generated from your RIDL schema
export const WebRPCSchemaHash = "15948e7930b709422fbfdf66eb70f4e51a54b37b"

//
// Types
//


export enum Service {
  NodeGateway = 'NodeGateway',
  API = 'API',
  Indexer = 'Indexer',
  Relayer = 'Relayer',
  Metadata = 'Metadata',
  Marketplace = 'Marketplace',
  Builder = 'Builder',
  WaaS = 'WaaS'
}

export enum EventType {
  FreeWarn = 'FreeWarn',
  FreeMax = 'FreeMax',
  OverWarn = 'OverWarn',
  OverMax = 'OverMax'
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
  freeWarn: number
  freeMax: number
  overWarn: number
  overMax: number
  blockTransactions: boolean
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

export interface ProjectStatus {
  projectId: number
  limit: Limit
  usageCounter: number
  ratelimitCounter: number
}

export interface Subscription {
  tier: string
}

export interface Minter {
  contracts: Array<string>
}

export interface ResourceAccess {
  projectId: number
  subscription: Subscription
  minter: Minter
}

export interface QuotaControl {
  getProjectStatus(args: GetProjectStatusArgs, headers?: object, signal?: AbortSignal): Promise<GetProjectStatusReturn>
  getAccessKey(args: GetAccessKeyArgs, headers?: object, signal?: AbortSignal): Promise<GetAccessKeyReturn>
  getDefaultAccessKey(args: GetDefaultAccessKeyArgs, headers?: object, signal?: AbortSignal): Promise<GetDefaultAccessKeyReturn>
  createAccessKey(args: CreateAccessKeyArgs, headers?: object, signal?: AbortSignal): Promise<CreateAccessKeyReturn>
  rotateAccessKey(args: RotateAccessKeyArgs, headers?: object, signal?: AbortSignal): Promise<RotateAccessKeyReturn>
  updateAccessKey(args: UpdateAccessKeyArgs, headers?: object, signal?: AbortSignal): Promise<UpdateAccessKeyReturn>
  updateDefaultAccessKey(args: UpdateDefaultAccessKeyArgs, headers?: object, signal?: AbortSignal): Promise<UpdateDefaultAccessKeyReturn>
  listAccessKeys(args: ListAccessKeysArgs, headers?: object, signal?: AbortSignal): Promise<ListAccessKeysReturn>
  disableAccessKey(args: DisableAccessKeyArgs, headers?: object, signal?: AbortSignal): Promise<DisableAccessKeyReturn>
  getProjectQuota(args: GetProjectQuotaArgs, headers?: object, signal?: AbortSignal): Promise<GetProjectQuotaReturn>
  getAccessQuota(args: GetAccessQuotaArgs, headers?: object, signal?: AbortSignal): Promise<GetAccessQuotaReturn>
  clearAccessQuotaCache(args: ClearAccessQuotaCacheArgs, headers?: object, signal?: AbortSignal): Promise<ClearAccessQuotaCacheReturn>
  getAccountUsage(args: GetAccountUsageArgs, headers?: object, signal?: AbortSignal): Promise<GetAccountUsageReturn>
  getAccessKeyUsage(args: GetAccessKeyUsageArgs, headers?: object, signal?: AbortSignal): Promise<GetAccessKeyUsageReturn>
  getAsyncUsage(args: GetAsyncUsageArgs, headers?: object, signal?: AbortSignal): Promise<GetAsyncUsageReturn>
  prepareUsage(args: PrepareUsageArgs, headers?: object, signal?: AbortSignal): Promise<PrepareUsageReturn>
  clearUsage(args: ClearUsageArgs, headers?: object, signal?: AbortSignal): Promise<ClearUsageReturn>
  notifyEvent(args: NotifyEventArgs, headers?: object, signal?: AbortSignal): Promise<NotifyEventReturn>
  updateProjectUsage(args: UpdateProjectUsageArgs, headers?: object, signal?: AbortSignal): Promise<UpdateProjectUsageReturn>
  updateKeyUsage(args: UpdateKeyUsageArgs, headers?: object, signal?: AbortSignal): Promise<UpdateKeyUsageReturn>
  updateUsage(args: UpdateUsageArgs, headers?: object, signal?: AbortSignal): Promise<UpdateUsageReturn>
  getUserPermission(args: GetUserPermissionArgs, headers?: object, signal?: AbortSignal): Promise<GetUserPermissionReturn>
}

export interface GetProjectStatusArgs {
  projectId: number
}

export interface GetProjectStatusReturn {
  projectStatus: ProjectStatus  
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
export interface GetProjectQuotaArgs {
  projectId: number
  now: string
}

export interface GetProjectQuotaReturn {
  accessQuota: AccessQuota  
}
export interface GetAccessQuotaArgs {
  accessKey: string
  now: string
}

export interface GetAccessQuotaReturn {
  accessQuota: AccessQuota  
}
export interface ClearAccessQuotaCacheArgs {
  projectID: number
}

export interface ClearAccessQuotaCacheReturn {
  ok: boolean  
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
export interface GetAsyncUsageArgs {
  projectID: number
  service?: Service
  from?: string
  to?: string
}

export interface GetAsyncUsageReturn {
  usage: AccessUsage  
}
export interface PrepareUsageArgs {
  projectID: number
  cycle: Cycle
  now: string
}

export interface PrepareUsageReturn {
  ok: boolean  
}
export interface ClearUsageArgs {
  projectID: number
  now: string
}

export interface ClearUsageReturn {
  ok: boolean  
}
export interface NotifyEventArgs {
  projectID: number
  eventType: EventType
}

export interface NotifyEventReturn {
  ok: boolean  
}
export interface UpdateProjectUsageArgs {
  service: Service
  now: string
  usage: {[key: number]: AccessUsage}
}

export interface UpdateProjectUsageReturn {
  ok: {[key: number]: boolean}  
}
export interface UpdateKeyUsageArgs {
  service: Service
  now: string
  usage: {[key: string]: AccessUsage}
}

export interface UpdateKeyUsageReturn {
  ok: {[key: string]: boolean}  
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
  resourceAccess: ResourceAccess  
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
  
  getProjectStatus = (args: GetProjectStatusArgs, headers?: object, signal?: AbortSignal): Promise<GetProjectStatusReturn> => {
    return this.fetch(
      this.url('GetProjectStatus'),
      createHTTPRequest(args, headers, signal)).then((res) => {
      return buildResponse(res).then(_data => {
        return {
          projectStatus: <ProjectStatus>(_data.projectStatus),
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
  
  getProjectQuota = (args: GetProjectQuotaArgs, headers?: object, signal?: AbortSignal): Promise<GetProjectQuotaReturn> => {
    return this.fetch(
      this.url('GetProjectQuota'),
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
  
  clearAccessQuotaCache = (args: ClearAccessQuotaCacheArgs, headers?: object, signal?: AbortSignal): Promise<ClearAccessQuotaCacheReturn> => {
    return this.fetch(
      this.url('ClearAccessQuotaCache'),
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
  
  getAsyncUsage = (args: GetAsyncUsageArgs, headers?: object, signal?: AbortSignal): Promise<GetAsyncUsageReturn> => {
    return this.fetch(
      this.url('GetAsyncUsage'),
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
  
  clearUsage = (args: ClearUsageArgs, headers?: object, signal?: AbortSignal): Promise<ClearUsageReturn> => {
    return this.fetch(
      this.url('ClearUsage'),
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
  
  updateProjectUsage = (args: UpdateProjectUsageArgs, headers?: object, signal?: AbortSignal): Promise<UpdateProjectUsageReturn> => {
    return this.fetch(
      this.url('UpdateProjectUsage'),
      createHTTPRequest(args, headers, signal)).then((res) => {
      return buildResponse(res).then(_data => {
        return {
          ok: <{[key: number]: boolean}>(_data.ok),
        }
      })
    }, (error) => {
      throw WebrpcRequestFailedError.new({ cause: `fetch(): ${error.message || ''}` })
    })
  }
  
  updateKeyUsage = (args: UpdateKeyUsageArgs, headers?: object, signal?: AbortSignal): Promise<UpdateKeyUsageReturn> => {
    return this.fetch(
      this.url('UpdateKeyUsage'),
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
          resourceAccess: <ResourceAccess>(_data.resourceAccess),
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

export class WebrpcClientDisconnectedError extends WebrpcError {
  constructor(
    name: string = 'WebrpcClientDisconnected',
    code: number = -8,
    message: string = 'client disconnected',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, WebrpcClientDisconnectedError.prototype)
  }
}

export class WebrpcStreamLostError extends WebrpcError {
  constructor(
    name: string = 'WebrpcStreamLost',
    code: number = -9,
    message: string = 'stream lost',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, WebrpcStreamLostError.prototype)
  }
}

export class WebrpcStreamFinishedError extends WebrpcError {
  constructor(
    name: string = 'WebrpcStreamFinished',
    code: number = -10,
    message: string = 'stream finished',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, WebrpcStreamFinishedError.prototype)
  }
}


// Schema errors

export class UnauthorizedError extends WebrpcError {
  constructor(
    name: string = 'Unauthorized',
    code: number = 1,
    message: string = 'Unauthorized access',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, UnauthorizedError.prototype)
  }
}

export class PermissionDeniedError extends WebrpcError {
  constructor(
    name: string = 'PermissionDenied',
    code: number = 2,
    message: string = 'Permission denied',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, PermissionDeniedError.prototype)
  }
}

export class SessionExpiredError extends WebrpcError {
  constructor(
    name: string = 'SessionExpired',
    code: number = 3,
    message: string = 'Session expired',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, SessionExpiredError.prototype)
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
    message: string = 'Quota limit exceeded',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, LimitExceededError.prototype)
  }
}

export class RateLimitError extends WebrpcError {
  constructor(
    name: string = 'RateLimit',
    code: number = 1003,
    message: string = 'Rate limit exceeded',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, RateLimitError.prototype)
  }
}

export class ProjectNotFoundError extends WebrpcError {
  constructor(
    name: string = 'ProjectNotFound',
    code: number = 1004,
    message: string = 'Project not found',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, ProjectNotFoundError.prototype)
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

export class AccessKeyMismatchError extends WebrpcError {
  constructor(
    name: string = 'AccessKeyMismatch',
    code: number = 1006,
    message: string = 'Access key does not belong to the project',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, AccessKeyMismatchError.prototype)
  }
}

export class InvalidOriginError extends WebrpcError {
  constructor(
    name: string = 'InvalidOrigin',
    code: number = 1007,
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
    code: number = 1008,
    message: string = 'Access key is not configured for this service',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, InvalidServiceError.prototype)
  }
}

export class UnauthorizedUserError extends WebrpcError {
  constructor(
    name: string = 'UnauthorizedUser',
    code: number = 1009,
    message: string = 'Unauthorized user',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, UnauthorizedUserError.prototype)
  }
}

export class NoDefaultKeyError extends WebrpcError {
  constructor(
    name: string = 'NoDefaultKey',
    code: number = 1011,
    message: string = 'No default access key found',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, NoDefaultKeyError.prototype)
  }
}

export class MaxAccessKeysError extends WebrpcError {
  constructor(
    name: string = 'MaxAccessKeys',
    code: number = 1012,
    message: string = 'Access keys limit reached',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, MaxAccessKeysError.prototype)
  }
}

export class AtLeastOneKeyError extends WebrpcError {
  constructor(
    name: string = 'AtLeastOneKey',
    code: number = 1013,
    message: string = 'There should be at least one active accessKey',
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
  WebrpcClientDisconnected = 'WebrpcClientDisconnected',
  WebrpcStreamLost = 'WebrpcStreamLost',
  WebrpcStreamFinished = 'WebrpcStreamFinished',
  Unauthorized = 'Unauthorized',
  PermissionDenied = 'PermissionDenied',
  SessionExpired = 'SessionExpired',
  Timeout = 'Timeout',
  LimitExceeded = 'LimitExceeded',
  RateLimit = 'RateLimit',
  ProjectNotFound = 'ProjectNotFound',
  AccessKeyNotFound = 'AccessKeyNotFound',
  AccessKeyMismatch = 'AccessKeyMismatch',
  InvalidOrigin = 'InvalidOrigin',
  InvalidService = 'InvalidService',
  UnauthorizedUser = 'UnauthorizedUser',
  NoDefaultKey = 'NoDefaultKey',
  MaxAccessKeys = 'MaxAccessKeys',
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
  [-8]: WebrpcClientDisconnectedError,
  [-9]: WebrpcStreamLostError,
  [-10]: WebrpcStreamFinishedError,
  [1]: UnauthorizedError,
  [2]: PermissionDeniedError,
  [3]: SessionExpiredError,
  [1001]: TimeoutError,
  [1002]: LimitExceededError,
  [1003]: RateLimitError,
  [1004]: ProjectNotFoundError,
  [1005]: AccessKeyNotFoundError,
  [1006]: AccessKeyMismatchError,
  [1007]: InvalidOriginError,
  [1008]: InvalidServiceError,
  [1009]: UnauthorizedUserError,
  [1011]: NoDefaultKeyError,
  [1012]: MaxAccessKeysError,
  [1013]: AtLeastOneKeyError,
}

export type Fetch = (input: RequestInfo, init?: RequestInit) => Promise<Response>
