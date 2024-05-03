/* eslint-disable */
// @ts-ignore
import * as _m0 from "protobufjs/minimal";

export const protobufPackage = "galactica.merkle";

/** QueryProofRequest is the request type for the Query.Proof method. */
export interface QueryProofRequest {
  /** registry  is the ZkCertificateRegistry hex address, which starts with 0x. */
  registry: string;
  /** leaf is the leaf uint256 value. */
  leaf: string;
}

/** QueryProofResponse is the response type for the Query.Proof method. */
export interface QueryProofResponse {
  /** proof is the merkle proof. */
  proof: Proof | undefined;
}

/** GetEmptyIndexRequest is the request type for the Query.GetEmptyIndex method. */
export interface GetEmptyIndexRequest {
  /** registry is the ZkCertificateRegistry hex address, which starts with 0x. */
  registry: string;
}

/** GetEmptyIndexResponse is the response type for the Query.GetEmptyIndex method. */
export interface GetEmptyIndexResponse {
  /** index is the empty leaf index. */
  index: number;
}

/** Proof is the merkle proof. */
export interface Proof {
  /** leaf is the leaf value encoded as a string containing the uint256 value. */
  leaf: string;
  /** path is the merkle proof path, encoded as a string containing the uint256 values. */
  path: string[];
  /** index is the leaf index. */
  index: number;
  /** root is the merkle root, value encoded as a string containing the uint256 value. */
  root: string;
}

function createBaseQueryProofRequest(): QueryProofRequest {
  return { registry: "", leaf: "" };
}

export const QueryProofRequest = {
  encode(message: QueryProofRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.registry !== "") {
      writer.uint32(10).string(message.registry);
    }
    if (message.leaf !== "") {
      writer.uint32(18).string(message.leaf);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): QueryProofRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseQueryProofRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.registry = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.leaf = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): QueryProofRequest {
    return {
      registry: isSet(object.registry) ? globalThis.String(object.registry) : "",
      leaf: isSet(object.leaf) ? globalThis.String(object.leaf) : "",
    };
  },

  toJSON(message: QueryProofRequest): unknown {
    const obj: any = {};
    if (message.registry !== "") {
      obj.registry = message.registry;
    }
    if (message.leaf !== "") {
      obj.leaf = message.leaf;
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<QueryProofRequest>, I>>(base?: I): QueryProofRequest {
    return QueryProofRequest.fromPartial(base ?? ({} as any));
  },
  fromPartial<I extends Exact<DeepPartial<QueryProofRequest>, I>>(object: I): QueryProofRequest {
    const message = createBaseQueryProofRequest();
    message.registry = object.registry ?? "";
    message.leaf = object.leaf ?? "";
    return message;
  },
};

function createBaseQueryProofResponse(): QueryProofResponse {
  return { proof: undefined };
}

export const QueryProofResponse = {
  encode(message: QueryProofResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.proof !== undefined) {
      Proof.encode(message.proof, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): QueryProofResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseQueryProofResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.proof = Proof.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): QueryProofResponse {
    return { proof: isSet(object.proof) ? Proof.fromJSON(object.proof) : undefined };
  },

  toJSON(message: QueryProofResponse): unknown {
    const obj: any = {};
    if (message.proof !== undefined) {
      obj.proof = Proof.toJSON(message.proof);
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<QueryProofResponse>, I>>(base?: I): QueryProofResponse {
    return QueryProofResponse.fromPartial(base ?? ({} as any));
  },
  fromPartial<I extends Exact<DeepPartial<QueryProofResponse>, I>>(object: I): QueryProofResponse {
    const message = createBaseQueryProofResponse();
    message.proof = (object.proof !== undefined && object.proof !== null) ? Proof.fromPartial(object.proof) : undefined;
    return message;
  },
};

function createBaseGetEmptyIndexRequest(): GetEmptyIndexRequest {
  return { registry: "" };
}

export const GetEmptyIndexRequest = {
  encode(message: GetEmptyIndexRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.registry !== "") {
      writer.uint32(10).string(message.registry);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): GetEmptyIndexRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseGetEmptyIndexRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.registry = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): GetEmptyIndexRequest {
    return { registry: isSet(object.registry) ? globalThis.String(object.registry) : "" };
  },

  toJSON(message: GetEmptyIndexRequest): unknown {
    const obj: any = {};
    if (message.registry !== "") {
      obj.registry = message.registry;
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<GetEmptyIndexRequest>, I>>(base?: I): GetEmptyIndexRequest {
    return GetEmptyIndexRequest.fromPartial(base ?? ({} as any));
  },
  fromPartial<I extends Exact<DeepPartial<GetEmptyIndexRequest>, I>>(object: I): GetEmptyIndexRequest {
    const message = createBaseGetEmptyIndexRequest();
    message.registry = object.registry ?? "";
    return message;
  },
};

function createBaseGetEmptyIndexResponse(): GetEmptyIndexResponse {
  return { index: 0 };
}

export const GetEmptyIndexResponse = {
  encode(message: GetEmptyIndexResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.index !== 0) {
      writer.uint32(8).uint32(message.index);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): GetEmptyIndexResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseGetEmptyIndexResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 8) {
            break;
          }

          message.index = reader.uint32();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): GetEmptyIndexResponse {
    return { index: isSet(object.index) ? globalThis.Number(object.index) : 0 };
  },

  toJSON(message: GetEmptyIndexResponse): unknown {
    const obj: any = {};
    if (message.index !== 0) {
      obj.index = Math.round(message.index);
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<GetEmptyIndexResponse>, I>>(base?: I): GetEmptyIndexResponse {
    return GetEmptyIndexResponse.fromPartial(base ?? ({} as any));
  },
  fromPartial<I extends Exact<DeepPartial<GetEmptyIndexResponse>, I>>(object: I): GetEmptyIndexResponse {
    const message = createBaseGetEmptyIndexResponse();
    message.index = object.index ?? 0;
    return message;
  },
};

function createBaseProof(): Proof {
  return { leaf: "", path: [], index: 0, root: "" };
}

export const Proof = {
  encode(message: Proof, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.leaf !== "") {
      writer.uint32(10).string(message.leaf);
    }
    for (const v of message.path) {
      writer.uint32(18).string(v!);
    }
    if (message.index !== 0) {
      writer.uint32(24).uint32(message.index);
    }
    if (message.root !== "") {
      writer.uint32(34).string(message.root);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Proof {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseProof();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.leaf = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.path.push(reader.string());
          continue;
        case 3:
          if (tag !== 24) {
            break;
          }

          message.index = reader.uint32();
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          message.root = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Proof {
    return {
      leaf: isSet(object.leaf) ? globalThis.String(object.leaf) : "",
      path: globalThis.Array.isArray(object?.path) ? object.path.map((e: any) => globalThis.String(e)) : [],
      index: isSet(object.index) ? globalThis.Number(object.index) : 0,
      root: isSet(object.root) ? globalThis.String(object.root) : "",
    };
  },

  toJSON(message: Proof): unknown {
    const obj: any = {};
    if (message.leaf !== "") {
      obj.leaf = message.leaf;
    }
    if (message.path?.length) {
      obj.path = message.path;
    }
    if (message.index !== 0) {
      obj.index = Math.round(message.index);
    }
    if (message.root !== "") {
      obj.root = message.root;
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<Proof>, I>>(base?: I): Proof {
    return Proof.fromPartial(base ?? ({} as any));
  },
  fromPartial<I extends Exact<DeepPartial<Proof>, I>>(object: I): Proof {
    const message = createBaseProof();
    message.leaf = object.leaf ?? "";
    message.path = object.path?.map((e) => e) || [];
    message.index = object.index ?? 0;
    message.root = object.root ?? "";
    return message;
  },
};

/** Query defines the gRPC querier service. */
export interface Query {
  /** Proof queries the proof of a leaf in the merkle tree. */
  Proof(request: QueryProofRequest): Promise<QueryProofResponse>;
  /** GetEmptyIndex queries the empty leaf index in the merkle tree. */
  GetEmptyIndex(request: GetEmptyIndexRequest): Promise<GetEmptyIndexResponse>;
}

export const QueryServiceName = "galactica.merkle.Query";
export class QueryClientImpl implements Query {
  private readonly rpc: Rpc;
  private readonly service: string;
  constructor(rpc: Rpc, opts?: { service?: string }) {
    this.service = opts?.service || QueryServiceName;
    this.rpc = rpc;
    this.Proof = this.Proof.bind(this);
    this.GetEmptyIndex = this.GetEmptyIndex.bind(this);
  }
  Proof(request: QueryProofRequest): Promise<QueryProofResponse> {
    const data = QueryProofRequest.encode(request).finish();
    const promise = this.rpc.request(this.service, "Proof", data);
    return promise.then((data) => QueryProofResponse.decode(_m0.Reader.create(data)));
  }

  GetEmptyIndex(request: GetEmptyIndexRequest): Promise<GetEmptyIndexResponse> {
    const data = GetEmptyIndexRequest.encode(request).finish();
    const promise = this.rpc.request(this.service, "GetEmptyIndex", data);
    return promise.then((data) => GetEmptyIndexResponse.decode(_m0.Reader.create(data)));
  }
}

interface Rpc {
  request(service: string, method: string, data: Uint8Array): Promise<Uint8Array>;
}

type Builtin = Date | Function | Uint8Array | string | number | boolean | undefined;

export type DeepPartial<T> = T extends Builtin ? T
  : T extends globalThis.Array<infer U> ? globalThis.Array<DeepPartial<U>>
  : T extends ReadonlyArray<infer U> ? ReadonlyArray<DeepPartial<U>>
  : T extends {} ? { [K in keyof T]?: DeepPartial<T[K]> }
  : Partial<T>;

type KeysOfUnion<T> = T extends T ? keyof T : never;
export type Exact<P, I extends P> = P extends Builtin ? P
  : P & { [K in keyof P]: Exact<P[K], I[K]> } & { [K in Exclude<keyof I, KeysOfUnion<P>>]: never };

function isSet(value: any): boolean {
  return value !== null && value !== undefined;
}
