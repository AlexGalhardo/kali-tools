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
	export class DiagnosticRequest {
	    record_type?: string;
	    kind: string;
	    target: string;
	    port: number;
	    timeout_ms: number;
	
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
	export class NmapAddress {
	    address: string;
	    type: string;
	
	    static createFrom(source: any = {}) {
	        return new NmapAddress(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.address = source["address"];
	        this.type = source["type"];
	    }
	}
	export class NmapFinished {
	    exit: string;
	
	    static createFrom(source: any = {}) {
	        return new NmapFinished(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.exit = source["exit"];
	    }
	}
	export class NmapService {
	    name: string;
	    product: string;
	    version: string;
	    method: string;
	
	    static createFrom(source: any = {}) {
	        return new NmapService(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.product = source["product"];
	        this.version = source["version"];
	        this.method = source["method"];
	    }
	}
	export class NmapPort {
	    service: NmapService;
	    port: number;
	    protocol: string;
	    state: NmapState;
	
	    static createFrom(source: any = {}) {
	        return new NmapPort(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.service = this.convertValues(source["service"], NmapService);
	        this.port = source["port"];
	        this.protocol = source["protocol"];
	        this.state = this.convertValues(source["state"], NmapState);
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
	export class NmapState {
	    state: string;
	
	    static createFrom(source: any = {}) {
	        return new NmapState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	    }
	}
	export class NmapHost {
	    status: NmapState;
	    addresses: NmapAddress[];
	    ports: NmapPort[];
	
	    static createFrom(source: any = {}) {
	        return new NmapHost(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = this.convertValues(source["status"], NmapState);
	        this.addresses = this.convertValues(source["addresses"], NmapAddress);
	        this.ports = this.convertValues(source["ports"], NmapPort);
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
	export class ServiceInspection {
	    scanner: string;
	    version: string;
	    finished: NmapFinished;
	    hosts: NmapHost[];
	
	    static createFrom(source: any = {}) {
	        return new ServiceInspection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scanner = source["scanner"];
	        this.version = source["version"];
	        this.finished = this.convertValues(source["finished"], NmapFinished);
	        this.hosts = this.convertValues(source["hosts"], NmapHost);
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

