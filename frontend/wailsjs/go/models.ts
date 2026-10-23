export namespace contract {
	
	export class Adapter {
	    index: number;
	    name: string;
	    gateways: string[];
	    dns: string[];
	
	    static createFrom(source: any = {}) {
	        return new Adapter(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.name = source["name"];
	        this.gateways = source["gateways"];
	        this.dns = source["dns"];
	    }
	}
	export class Capability {
	    id: string;
	    state: string;
	    detail: string;
	    provider: string;
	
	    static createFrom(source: any = {}) {
	        return new Capability(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.state = source["state"];
	        this.detail = source["detail"];
	        this.provider = source["provider"];
	    }
	}
	export class Packet {
	    number: number;
	    // Go type: time
	    timestamp: any;
	    length: number;
	    source: string;
	    destination: string;
	    protocol: string;
	    info: string;
	
	    static createFrom(source: any = {}) {
	        return new Packet(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.number = source["number"];
	        this.timestamp = this.convertValues(source["timestamp"], null);
	        this.length = source["length"];
	        this.source = source["source"];
	        this.destination = source["destination"];
	        this.protocol = source["protocol"];
	        this.info = source["info"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Capture {
	    packets: Packet[];
	    limited: boolean;
	    format: string;
	
	    static createFrom(source: any = {}) {
	        return new Capture(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.packets = this.convertValues(source["packets"], Packet);
	        this.limited = source["limited"];
	        this.format = source["format"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Connection {
	    protocol: string;
	    local: string;
	    remote: string;
	    state: string;
	    pid: number;
	    process: string;
	
	    static createFrom(source: any = {}) {
	        return new Connection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.protocol = source["protocol"];
	        this.local = source["local"];
	        this.remote = source["remote"];
	        this.state = source["state"];
	        this.pid = source["pid"];
	        this.process = source["process"];
	    }
	}
	export class ConnectionTable {
	    connections: Connection[];
	    limited: boolean;
	    provider: string;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionTable(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connections = this.convertValues(source["connections"], Connection);
	        this.limited = source["limited"];
	        this.provider = source["provider"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Counter {
	    id: string;
	    received: number;
	    sent: number;
	
	    static createFrom(source: any = {}) {
	        return new Counter(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.received = source["received"];
	        this.sent = source["sent"];
	    }
	}
	export class DNSAuthServer {
	    ns: string;
	    address: string;
	    status: string;
	    serial?: number;
	    ns_set: string[];
	    aa: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DNSAuthServer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ns = source["ns"];
	        this.address = source["address"];
	        this.status = source["status"];
	        this.serial = source["serial"];
	        this.ns_set = source["ns_set"];
	        this.aa = source["aa"];
	    }
	}
	export class DNSResolverAnswer {
	    resolver: string;
	    transport: string;
	    status: string;
	    rcode?: string;
	    ad: boolean;
	    answers: string[];
	    min_ttl?: number;
	    elapsed_ms: number;
	
	    static createFrom(source: any = {}) {
	        return new DNSResolverAnswer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.resolver = source["resolver"];
	        this.transport = source["transport"];
	        this.status = source["status"];
	        this.rcode = source["rcode"];
	        this.ad = source["ad"];
	        this.answers = source["answers"];
	        this.min_ttl = source["min_ttl"];
	        this.elapsed_ms = source["elapsed_ms"];
	    }
	}
	export class DNSComparison {
	    results: DNSResolverAnswer[];
	    agree: boolean;
	    groups: string[][];
	
	    static createFrom(source: any = {}) {
	        return new DNSComparison(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.results = this.convertValues(source["results"], DNSResolverAnswer);
	        this.agree = source["agree"];
	        this.groups = source["groups"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DNSConsistency {
	    zone: string;
	    servers: DNSAuthServer[];
	    serials_agree: boolean;
	    ns_sets_agree: boolean;
	    verdict: string;
	
	    static createFrom(source: any = {}) {
	        return new DNSConsistency(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.zone = source["zone"];
	        this.servers = this.convertValues(source["servers"], DNSAuthServer);
	        this.serials_agree = source["serials_agree"];
	        this.ns_sets_agree = source["ns_sets_agree"];
	        this.verdict = source["verdict"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DNSEncryptedResult {
	    endpoint: string;
	    protocol: string;
	    status: string;
	    http_code?: number;
	    rcode?: string;
	    elapsed_ms: number;
	
	    static createFrom(source: any = {}) {
	        return new DNSEncryptedResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.endpoint = source["endpoint"];
	        this.protocol = source["protocol"];
	        this.status = source["status"];
	        this.http_code = source["http_code"];
	        this.rcode = source["rcode"];
	        this.elapsed_ms = source["elapsed_ms"];
	    }
	}
	export class DNSSECLink {
	    zone: string;
	    ds_matches: boolean;
	    key_tag: number;
	    algorithm: string;
	    sig_valid: boolean;
	    status: string;
	    detail?: string;
	
	    static createFrom(source: any = {}) {
	        return new DNSSECLink(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.zone = source["zone"];
	        this.ds_matches = source["ds_matches"];
	        this.key_tag = source["key_tag"];
	        this.algorithm = source["algorithm"];
	        this.sig_valid = source["sig_valid"];
	        this.status = source["status"];
	        this.detail = source["detail"];
	    }
	}
	export class DNSSECChain {
	    links: DNSSECLink[];
	    answer_sig: boolean;
	    verdict: string;
	    resolver: string;
	
	    static createFrom(source: any = {}) {
	        return new DNSSECChain(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.links = this.convertValues(source["links"], DNSSECLink);
	        this.answer_sig = source["answer_sig"];
	        this.verdict = source["verdict"];
	        this.resolver = source["resolver"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DNSInspectReport {
	    name: string;
	    type: string;
	    compare?: DNSComparison;
	    soa_ns?: DNSConsistency;
	    dnssec?: DNSSECChain;
	    encrypted?: DNSEncryptedResult[];
	    queries: number;
	    limited: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DNSInspectReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.compare = this.convertValues(source["compare"], DNSComparison);
	        this.soa_ns = this.convertValues(source["soa_ns"], DNSConsistency);
	        this.dnssec = this.convertValues(source["dnssec"], DNSSECChain);
	        this.encrypted = this.convertValues(source["encrypted"], DNSEncryptedResult);
	        this.queries = source["queries"];
	        this.limited = source["limited"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DNSInspectRequest {
	    name: string;
	    type: string;
	    resolvers?: string[];
	    doh?: string[];
	    dot?: string[];
	    checks: string[];
	    trust_anchors?: string[];
	    timeout_ms: number;
	
	    static createFrom(source: any = {}) {
	        return new DNSInspectRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.resolvers = source["resolvers"];
	        this.doh = source["doh"];
	        this.dot = source["dot"];
	        this.checks = source["checks"];
	        this.trust_anchors = source["trust_anchors"];
	        this.timeout_ms = source["timeout_ms"];
	    }
	}
	
	
	
	export class DiagnosticRequest {
	    record_type?: string;
	    kind: string;
	    target: string;
	    port: number;
	    timeout_ms: number;
	    redirects?: number;
	
	    static createFrom(source: any = {}) {
	        return new DiagnosticRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.record_type = source["record_type"];
	        this.kind = source["kind"];
	        this.target = source["target"];
	        this.port = source["port"];
	        this.timeout_ms = source["timeout_ms"];
	        this.redirects = source["redirects"];
	    }
	}
	export class RedirectHop {
	    url: string;
	    status_code?: number;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new RedirectHop(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.status_code = source["status_code"];
	        this.status = source["status"];
	    }
	}
	export class RedirectTarget {
	    location?: string;
	    valid: boolean;
	    self_loop: boolean;
	    cross_host: boolean;
	    downgrade: boolean;
	    notes?: string[];
	
	    static createFrom(source: any = {}) {
	        return new RedirectTarget(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.location = source["location"];
	        this.valid = source["valid"];
	        this.self_loop = source["self_loop"];
	        this.cross_host = source["cross_host"];
	        this.downgrade = source["downgrade"];
	        this.notes = source["notes"];
	    }
	}
	export class SecurityHeader {
	    name: string;
	    present: boolean;
	    value?: string;
	    notes?: string[];
	
	    static createFrom(source: any = {}) {
	        return new SecurityHeader(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.present = source["present"];
	        this.value = source["value"];
	        this.notes = source["notes"];
	    }
	}
	export class HTTPPhases {
	    dns_ms?: number;
	    connect_ms?: number;
	    tls_ms?: number;
	    ttfb_ms?: number;
	    total_ms: number;
	
	    static createFrom(source: any = {}) {
	        return new HTTPPhases(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dns_ms = source["dns_ms"];
	        this.connect_ms = source["connect_ms"];
	        this.tls_ms = source["tls_ms"];
	        this.ttfb_ms = source["ttfb_ms"];
	        this.total_ms = source["total_ms"];
	    }
	}
	export class HTTPMetadata {
	    status_code?: number;
	    final_url?: string;
	    phases: HTTPPhases;
	    security_headers: SecurityHeader[];
	    redirect?: RedirectTarget;
	    chain?: RedirectHop[];
	    chain_status?: string;
	
	    static createFrom(source: any = {}) {
	        return new HTTPMetadata(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status_code = source["status_code"];
	        this.final_url = source["final_url"];
	        this.phases = this.convertValues(source["phases"], HTTPPhases);
	        this.security_headers = this.convertValues(source["security_headers"], SecurityHeader);
	        this.redirect = this.convertValues(source["redirect"], RedirectTarget);
	        this.chain = this.convertValues(source["chain"], RedirectHop);
	        this.chain_status = source["chain_status"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DiagnosticResult {
	    error_code?: string;
	    kind: string;
	    target: string;
	    status: string;
	    elapsed_ms: number;
	    values: string[];
	    provider: string;
	    http?: HTTPMetadata;
	
	    static createFrom(source: any = {}) {
	        return new DiagnosticResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.error_code = source["error_code"];
	        this.kind = source["kind"];
	        this.target = source["target"];
	        this.status = source["status"];
	        this.elapsed_ms = source["elapsed_ms"];
	        this.values = source["values"];
	        this.provider = source["provider"];
	        this.http = this.convertValues(source["http"], HTTPMetadata);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class InspectionAddress {
	    address: string;
	    type: string;
	
	    static createFrom(source: any = {}) {
	        return new InspectionAddress(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.address = source["address"];
	        this.type = source["type"];
	    }
	}
	export class InspectionFinished {
	    exit: string;
	
	    static createFrom(source: any = {}) {
	        return new InspectionFinished(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.exit = source["exit"];
	    }
	}
	export class InspectionService {
	    name: string;
	    product: string;
	    version: string;
	    method: string;
	
	    static createFrom(source: any = {}) {
	        return new InspectionService(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.product = source["product"];
	        this.version = source["version"];
	        this.method = source["method"];
	    }
	}
	export class InspectionPort {
	    service: InspectionService;
	    port: number;
	    protocol: string;
	    state: InspectionState;
	
	    static createFrom(source: any = {}) {
	        return new InspectionPort(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.service = this.convertValues(source["service"], InspectionService);
	        this.port = source["port"];
	        this.protocol = source["protocol"];
	        this.state = this.convertValues(source["state"], InspectionState);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class InspectionState {
	    state: string;
	
	    static createFrom(source: any = {}) {
	        return new InspectionState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	    }
	}
	export class InspectionHost {
	    status: InspectionState;
	    addresses: InspectionAddress[];
	    ports: InspectionPort[];
	
	    static createFrom(source: any = {}) {
	        return new InspectionHost(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = this.convertValues(source["status"], InspectionState);
	        this.addresses = this.convertValues(source["addresses"], InspectionAddress);
	        this.ports = this.convertValues(source["ports"], InspectionPort);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	export class Interface {
	    index: number;
	    name: string;
	    mac: string;
	    mtu: number;
	    up: boolean;
	    loopback: boolean;
	    addresses: string[];
	
	    static createFrom(source: any = {}) {
	        return new Interface(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.name = source["name"];
	        this.mac = source["mac"];
	        this.mtu = source["mtu"];
	        this.up = source["up"];
	        this.loopback = source["loopback"];
	        this.addresses = source["addresses"];
	    }
	}
	export class Inventory {
	    adapters: Adapter[];
	    counters: Counter[];
	
	    static createFrom(source: any = {}) {
	        return new Inventory(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.adapters = this.convertValues(source["adapters"], Adapter);
	        this.counters = this.convertValues(source["counters"], Counter);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Job {
	    id: string;
	    kind: string;
	    state: string;
	    // Go type: time
	    created: any;
	    // Go type: time
	    finished?: any;
	    summary: string;
	
	    static createFrom(source: any = {}) {
	        return new Job(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.state = source["state"];
	        this.created = this.convertValues(source["created"], null);
	        this.finished = this.convertValues(source["finished"], null);
	        this.summary = source["summary"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class LiveCaptureRequest {
	    backend?: string;
	    address: string;
	    duration_seconds: number;
	    packet_limit: number;
	
	    static createFrom(source: any = {}) {
	        return new LiveCaptureRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.backend = source["backend"];
	        this.address = source["address"];
	        this.duration_seconds = source["duration_seconds"];
	        this.packet_limit = source["packet_limit"];
	    }
	}
	export class LocalScope {
	    interface: string;
	    cidr: string;
	
	    static createFrom(source: any = {}) {
	        return new LocalScope(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.interface = source["interface"];
	        this.cidr = source["cidr"];
	    }
	}
	export class LocalDiscoveryRequest {
	    scope: LocalScope;
	    protocols: string[];
	    duration_seconds: number;
	    service_types?: string[];
	    ssdp_target?: string;
	
	    static createFrom(source: any = {}) {
	        return new LocalDiscoveryRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scope = this.convertValues(source["scope"], LocalScope);
	        this.protocols = source["protocols"];
	        this.duration_seconds = source["duration_seconds"];
	        this.service_types = source["service_types"];
	        this.ssdp_target = source["ssdp_target"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class LocalResponder {
	    protocol: string;
	    address: string;
	    port?: number;
	    name: string;
	    host?: string;
	    service_type?: string;
	    txt?: string[];
	    server?: string;
	    location?: string;
	
	    static createFrom(source: any = {}) {
	        return new LocalResponder(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.protocol = source["protocol"];
	        this.address = source["address"];
	        this.port = source["port"];
	        this.name = source["name"];
	        this.host = source["host"];
	        this.service_type = source["service_type"];
	        this.txt = source["txt"];
	        this.server = source["server"];
	        this.location = source["location"];
	    }
	}
	export class LocalDiscoveryResult {
	    responders: LocalResponder[];
	    limited: boolean;
	    out_of_scope: number;
	    malformed: number;
	    errors: string[];
	
	    static createFrom(source: any = {}) {
	        return new LocalDiscoveryResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.responders = this.convertValues(source["responders"], LocalResponder);
	        this.limited = source["limited"];
	        this.out_of_scope = source["out_of_scope"];
	        this.malformed = source["malformed"];
	        this.errors = source["errors"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class Neighbor {
	    address: string;
	    mac: string;
	    family: string;
	    interface_index: number;
	    interface_name: string;
	    state: string;
	    router: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Neighbor(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.address = source["address"];
	        this.mac = source["mac"];
	        this.family = source["family"];
	        this.interface_index = source["interface_index"];
	        this.interface_name = source["interface_name"];
	        this.state = source["state"];
	        this.router = source["router"];
	    }
	}
	export class NeighborTable {
	    neighbors: Neighbor[];
	    limited: boolean;
	    provider: string;
	
	    static createFrom(source: any = {}) {
	        return new NeighborTable(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.neighbors = this.convertValues(source["neighbors"], Neighbor);
	        this.limited = source["limited"];
	        this.provider = source["provider"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class PortResult {
	    host: string;
	    port: number;
	    state: string;
	
	    static createFrom(source: any = {}) {
	        return new PortResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.host = source["host"];
	        this.port = source["port"];
	        this.state = source["state"];
	    }
	}
	
	
	export class SNMPInterface {
	    index: number;
	    descr: string;
	    oper_status: string;
	    speed_bps: number;
	    mac: string;
	
	    static createFrom(source: any = {}) {
	        return new SNMPInterface(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.descr = source["descr"];
	        this.oper_status = source["oper_status"];
	        this.speed_bps = source["speed_bps"];
	        this.mac = source["mac"];
	    }
	}
	export class SNMPv3Request {
	    user: string;
	    auth_protocol: string;
	    auth_pass?: string;
	    priv_protocol?: string;
	    priv_pass?: string;
	
	    static createFrom(source: any = {}) {
	        return new SNMPv3Request(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.user = source["user"];
	        this.auth_protocol = source["auth_protocol"];
	        this.auth_pass = source["auth_pass"];
	        this.priv_protocol = source["priv_protocol"];
	        this.priv_pass = source["priv_pass"];
	    }
	}
	export class SNMPRequest {
	    scope: LocalScope;
	    target: string;
	    port: number;
	    owned: boolean;
	    version: string;
	    community?: string;
	    v3?: SNMPv3Request;
	    timeout_ms: number;
	    interfaces: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SNMPRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scope = this.convertValues(source["scope"], LocalScope);
	        this.target = source["target"];
	        this.port = source["port"];
	        this.owned = source["owned"];
	        this.version = source["version"];
	        this.community = source["community"];
	        this.v3 = this.convertValues(source["v3"], SNMPv3Request);
	        this.timeout_ms = source["timeout_ms"];
	        this.interfaces = source["interfaces"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SNMPResult {
	    target: string;
	    system: Record<string, string>;
	    interfaces: SNMPInterface[];
	    limited: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SNMPResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.target = source["target"];
	        this.system = source["system"];
	        this.interfaces = this.convertValues(source["interfaces"], SNMPInterface);
	        this.limited = source["limited"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class ServiceInspection {
	    scanner: string;
	    version: string;
	    finished: InspectionFinished;
	    hosts: InspectionHost[];
	
	    static createFrom(source: any = {}) {
	        return new ServiceInspection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scanner = source["scanner"];
	        this.version = source["version"];
	        this.finished = this.convertValues(source["finished"], InspectionFinished);
	        this.hosts = this.convertValues(source["hosts"], InspectionHost);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Settings {
	    language: string;
	    retention_days: number;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.language = source["language"];
	        this.retention_days = source["retention_days"];
	    }
	}
	export class TLSAuditCertificate {
	    index: number;
	    subject: string;
	    issuer: string;
	    serial_hex: string;
	    // Go type: time
	    not_before: any;
	    // Go type: time
	    not_after: any;
	    days_remaining: number;
	    sans: string[];
	    sans_limited: boolean;
	    key_algorithm: string;
	    key_bits: number;
	    signature_algorithm: string;
	    is_ca: boolean;
	    self_signed: boolean;
	    aia_issuers: string[];
	    ocsp_servers: string[];
	    crl_distribution_points: string[];
	    embedded_scts: number;
	    sha256: string;
	
	    static createFrom(source: any = {}) {
	        return new TLSAuditCertificate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.subject = source["subject"];
	        this.issuer = source["issuer"];
	        this.serial_hex = source["serial_hex"];
	        this.not_before = this.convertValues(source["not_before"], null);
	        this.not_after = this.convertValues(source["not_after"], null);
	        this.days_remaining = source["days_remaining"];
	        this.sans = source["sans"];
	        this.sans_limited = source["sans_limited"];
	        this.key_algorithm = source["key_algorithm"];
	        this.key_bits = source["key_bits"];
	        this.signature_algorithm = source["signature_algorithm"];
	        this.is_ca = source["is_ca"];
	        this.self_signed = source["self_signed"];
	        this.aia_issuers = source["aia_issuers"];
	        this.ocsp_servers = source["ocsp_servers"];
	        this.crl_distribution_points = source["crl_distribution_points"];
	        this.embedded_scts = source["embedded_scts"];
	        this.sha256 = source["sha256"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TLSAuditFinding {
	    source: string;
	    id: string;
	    severity: string;
	    cert_index: number;
	    detail?: string;
	
	    static createFrom(source: any = {}) {
	        return new TLSAuditFinding(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.id = source["id"];
	        this.severity = source["severity"];
	        this.cert_index = source["cert_index"];
	        this.detail = source["detail"];
	    }
	}
	export class TLSAuditProtocolResult {
	    version: string;
	    status: string;
	    cipher?: string;
	
	    static createFrom(source: any = {}) {
	        return new TLSAuditProtocolResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.status = source["status"];
	        this.cipher = source["cipher"];
	    }
	}
	export class TLSAuditSCTSummary {
	    tls_extension: number;
	    embedded: number;
	
	    static createFrom(source: any = {}) {
	        return new TLSAuditSCTSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tls_extension = source["tls_extension"];
	        this.embedded = source["embedded"];
	    }
	}
	export class TLSAuditReport {
	    host: string;
	    port: number;
	    server_name: string;
	    negotiated_version: string;
	    cipher_suite: string;
	    alpn: string;
	    verified: boolean;
	    verify_error?: string;
	    verifier: string;
	    ocsp_stapled: boolean;
	    scts: TLSAuditSCTSummary;
	    chain: TLSAuditCertificate[];
	    protocols: TLSAuditProtocolResult[];
	    findings: TLSAuditFinding[];
	    findings_limited: boolean;
	    lint_engine?: string;
	
	    static createFrom(source: any = {}) {
	        return new TLSAuditReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.host = source["host"];
	        this.port = source["port"];
	        this.server_name = source["server_name"];
	        this.negotiated_version = source["negotiated_version"];
	        this.cipher_suite = source["cipher_suite"];
	        this.alpn = source["alpn"];
	        this.verified = source["verified"];
	        this.verify_error = source["verify_error"];
	        this.verifier = source["verifier"];
	        this.ocsp_stapled = source["ocsp_stapled"];
	        this.scts = this.convertValues(source["scts"], TLSAuditSCTSummary);
	        this.chain = this.convertValues(source["chain"], TLSAuditCertificate);
	        this.protocols = this.convertValues(source["protocols"], TLSAuditProtocolResult);
	        this.findings = this.convertValues(source["findings"], TLSAuditFinding);
	        this.findings_limited = source["findings_limited"];
	        this.lint_engine = source["lint_engine"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TLSAuditRequest {
	    host: string;
	    port: number;
	    server_name?: string;
	    timeout_ms: number;
	    lint: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TLSAuditRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.host = source["host"];
	        this.port = source["port"];
	        this.server_name = source["server_name"];
	        this.timeout_ms = source["timeout_ms"];
	        this.lint = source["lint"];
	    }
	}
	
	export class ThroughputSample {
	    endpoint: string;
	    duration_ms: number;
	    byte_count: number;
	    source: string;
	
	    static createFrom(source: any = {}) {
	        return new ThroughputSample(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.endpoint = source["endpoint"];
	        this.duration_ms = source["duration_ms"];
	        this.byte_count = source["byte_count"];
	        this.source = source["source"];
	    }
	}
	export class Tool {
	    name: string;
	    state: string;
	    version: string;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new Tool(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.state = source["state"];
	        this.version = source["version"];
	        this.path = source["path"];
	    }
	}
	export class WiFi {
	    channel?: number;
	    profile: string;
	    ssid: string;
	    bssid: string;
	    signal: number;
	    authentication: number;
	    state: string;
	
	    static createFrom(source: any = {}) {
	        return new WiFi(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.channel = source["channel"];
	        this.profile = source["profile"];
	        this.ssid = source["ssid"];
	        this.bssid = source["bssid"];
	        this.signal = source["signal"];
	        this.authentication = source["authentication"];
	        this.state = source["state"];
	    }
	}
	export class WiFiSecret {
	    state: string;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new WiFiSecret(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.value = source["value"];
	    }
	}

}

