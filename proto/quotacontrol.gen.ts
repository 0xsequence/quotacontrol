/* eslint-disable */
// quota-control v0.19.2 1c10f550ea5e6b885e6e95e914f34192b968c718
// --
// Code generated by webrpc-gen@v0.22.0 with typescript@v0.16.1 generator. DO NOT EDIT.
//
// webrpc-gen -schema=quotacontrol.ridl -target=typescript@v0.16.1 -client -out=./quotacontrol.gen.ts

export const WebrpcHeader = "Webrpc"

export const WebrpcHeaderValue = "webrpc@v0.22.0;gen-typescript@v0.16.1;quota-control@v0.19.2"

// WebRPC description and code-gen version
export const WebRPCVersion = "v1"

// Schema version of your RIDL schema
export const WebRPCSchemaVersion = "v0.19.2"

// Schema hash generated from your RIDL schema
export const WebRPCSchemaHash = "1c10f550ea5e6b885e6e95e914f34192b968c718"

type WebrpcGenVersions = {
  webrpcGenVersion: string;
  codeGenName: string;
  codeGenVersion: string;
  schemaName: string;
  schemaVersion: string;
};

export function VersionFromHeader(headers: Headers): WebrpcGenVersions {
  const headerValue = headers.get(WebrpcHeader);
  if (!headerValue) {
    return {
      webrpcGenVersion: "",
      codeGenName: "",
      codeGenVersion: "",
      schemaName: "",
      schemaVersion: "",
    };
  }

  return parseWebrpcGenVersions(headerValue);
}

function parseWebrpcGenVersions(header: string): WebrpcGenVersions {
  const versions = header.split(";");
  if (versions.length < 3) {
    return {
      webrpcGenVersion: "",
      codeGenName: "",
      codeGenVersion: "",
      schemaName: "",
      schemaVersion: "",
    };
  }

  const [_, webrpcGenVersion] = versions[0].split("@");
  const [codeGenName, codeGenVersion] = versions[1].split("@");
  const [schemaName, schemaVersion] = versions[2].split("@");

  return {
    webrpcGenVersion,
    codeGenName,
    codeGenVersion,
    schemaName,
    schemaVersion,
  };
}

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
  chainIds: Array<number>
  displayName: string
  accessKey: string
  active: boolean
  default: boolean
  requireOrigin: boolean
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
  requireOrigin: boolean
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
  requireOrigin?: boolean
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
    this.hostname = hostname.replace(/\/*$/, '')
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
  const reqHeaders: {[key: string]: string} = { ...headers, 'Content-Type': 'application/json' }
  reqHeaders[WebrpcHeader] = WebrpcHeaderValue

  return {
    method: 'POST',
    headers: reqHeaders,
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
    code: number = 1000,
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
    code: number = 1001,
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
    code: number = 1002,
    message: string = 'Session expired',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, SessionExpiredError.prototype)
  }
}

export class MethodNotFoundError extends WebrpcError {
  constructor(
    name: string = 'MethodNotFound',
    code: number = 1003,
    message: string = 'Method not found',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, MethodNotFoundError.prototype)
  }
}

export class RequestConflictError extends WebrpcError {
  constructor(
    name: string = 'RequestConflict',
    code: number = 1004,
    message: string = 'Conflict with target resource',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, RequestConflictError.prototype)
  }
}

export class AbortedError extends WebrpcError {
  constructor(
    name: string = 'Aborted',
    code: number = 1005,
    message: string = 'Request aborted',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, AbortedError.prototype)
  }
}

export class GeoblockedError extends WebrpcError {
  constructor(
    name: string = 'Geoblocked',
    code: number = 1006,
    message: string = 'Geoblocked region',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, GeoblockedError.prototype)
  }
}

export class RateLimitedError extends WebrpcError {
  constructor(
    name: string = 'RateLimited',
    code: number = 1007,
    message: string = 'Rate-limited. Please slow down.',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, RateLimitedError.prototype)
  }
}

export class ProjectNotFoundError extends WebrpcError {
  constructor(
    name: string = 'ProjectNotFound',
    code: number = 1008,
    message: string = 'Project not found',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, ProjectNotFoundError.prototype)
  }
}

export class SecretKeyCorsDisallowedError extends WebrpcError {
  constructor(
    name: string = 'SecretKeyCorsDisallowed',
    code: number = 1009,
    message: string = 'CORS disallowed. Admin API Secret Key can't be used from a web app.',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, SecretKeyCorsDisallowedError.prototype)
  }
}

export class AccessKeyNotFoundError extends WebrpcError {
  constructor(
    name: string = 'AccessKeyNotFound',
    code: number = 1101,
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
    code: number = 1102,
    message: string = 'Access key mismatch',
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
    code: number = 1103,
    message: string = 'Invalid origin for Access Key',
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
    code: number = 1104,
    message: string = 'Service not enabled for Access key',
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
    code: number = 1105,
    message: string = 'Unauthorized user',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, UnauthorizedUserError.prototype)
  }
}

export class InvalidChainError extends WebrpcError {
  constructor(
    name: string = 'InvalidChain',
    code: number = 1106,
    message: string = 'Network not enabled for Access key',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, InvalidChainError.prototype)
  }
}

export class QuotaExceededError extends WebrpcError {
  constructor(
    name: string = 'QuotaExceeded',
    code: number = 1200,
    message: string = 'Quota request exceeded',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, QuotaExceededError.prototype)
  }
}

export class QuotaRateLimitError extends WebrpcError {
  constructor(
    name: string = 'QuotaRateLimit',
    code: number = 1201,
    message: string = 'Quota rate limit exceeded',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, QuotaRateLimitError.prototype)
  }
}

export class NoDefaultKeyError extends WebrpcError {
  constructor(
    name: string = 'NoDefaultKey',
    code: number = 1300,
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
    code: number = 1301,
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
    code: number = 1302,
    message: string = 'You need at least one Access Key',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, AtLeastOneKeyError.prototype)
  }
}

export class TimeoutError extends WebrpcError {
  constructor(
    name: string = 'Timeout',
    code: number = 1900,
    message: string = 'Request timed out',
    status: number = 0,
    cause?: string
  ) {
    super(name, code, message, status, cause)
    Object.setPrototypeOf(this, TimeoutError.prototype)
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
  MethodNotFound = 'MethodNotFound',
  RequestConflict = 'RequestConflict',
  Aborted = 'Aborted',
  Geoblocked = 'Geoblocked',
  RateLimited = 'RateLimited',
  ProjectNotFound = 'ProjectNotFound',
  SecretKeyCorsDisallowed = 'SecretKeyCorsDisallowed',
  AccessKeyNotFound = 'AccessKeyNotFound',
  AccessKeyMismatch = 'AccessKeyMismatch',
  InvalidOrigin = 'InvalidOrigin',
  InvalidService = 'InvalidService',
  UnauthorizedUser = 'UnauthorizedUser',
  InvalidChain = 'InvalidChain',
  QuotaExceeded = 'QuotaExceeded',
  QuotaRateLimit = 'QuotaRateLimit',
  NoDefaultKey = 'NoDefaultKey',
  MaxAccessKeys = 'MaxAccessKeys',
  AtLeastOneKey = 'AtLeastOneKey',
  Timeout = 'Timeout',
}

export enum WebrpcErrorCodes {
  WebrpcEndpoint = 0,
  WebrpcRequestFailed = -1,
  WebrpcBadRoute = -2,
  WebrpcBadMethod = -3,
  WebrpcBadRequest = -4,
  WebrpcBadResponse = -5,
  WebrpcServerPanic = -6,
  WebrpcInternalError = -7,
  WebrpcClientDisconnected = -8,
  WebrpcStreamLost = -9,
  WebrpcStreamFinished = -10,
  Unauthorized = 1000,
  PermissionDenied = 1001,
  SessionExpired = 1002,
  MethodNotFound = 1003,
  RequestConflict = 1004,
  Aborted = 1005,
  Geoblocked = 1006,
  RateLimited = 1007,
  ProjectNotFound = 1008,
  SecretKeyCorsDisallowed = 1009,
  AccessKeyNotFound = 1101,
  AccessKeyMismatch = 1102,
  InvalidOrigin = 1103,
  InvalidService = 1104,
  UnauthorizedUser = 1105,
  InvalidChain = 1106,
  QuotaExceeded = 1200,
  QuotaRateLimit = 1201,
  NoDefaultKey = 1300,
  MaxAccessKeys = 1301,
  AtLeastOneKey = 1302,
  Timeout = 1900,
}

export const webrpcErrorByCode: { [code: number]: any } = {
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
  [1000]: UnauthorizedError,
  [1001]: PermissionDeniedError,
  [1002]: SessionExpiredError,
  [1003]: MethodNotFoundError,
  [1004]: RequestConflictError,
  [1005]: AbortedError,
  [1006]: GeoblockedError,
  [1007]: RateLimitedError,
  [1008]: ProjectNotFoundError,
  [1009]: SecretKeyCorsDisallowedError,
  [1101]: AccessKeyNotFoundError,
  [1102]: AccessKeyMismatchError,
  [1103]: InvalidOriginError,
  [1104]: InvalidServiceError,
  [1105]: UnauthorizedUserError,
  [1106]: InvalidChainError,
  [1200]: QuotaExceededError,
  [1201]: QuotaRateLimitError,
  [1300]: NoDefaultKeyError,
  [1301]: MaxAccessKeysError,
  [1302]: AtLeastOneKeyError,
  [1900]: TimeoutError,
}

export type Fetch = (input: RequestInfo, init?: RequestInit) => Promise<Response>

