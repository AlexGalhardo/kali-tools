export namespace capture {
	
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
	export class ImportResult {
	    packets: Packet[];
	    limited: boolean;
	    format: string;
	
	    static createFrom(source: any = {}) {
	        return new ImportResult(source);
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
	export class LiveRequest {
	    address: string;
	    duration_seconds: number;
	    packet_limit: number;
	
	    static createFrom(source: any = {}) {
	        return new LiveRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.address = source["address"];
	        this.duration_seconds = source["duration_seconds"];
	        this.packet_limit = source["packet_limit"];
	    }
	}

}

export namespace domain {
	
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

}

export namespace network {
	
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
	export class InterfaceInfo {
	    index: number;
	    name: string;
	    mac: string;
	    mtu: number;
	    up: boolean;
	    loopback: boolean;
	    addresses: string[];
	
	    static createFrom(source: any = {}) {
	        return new InterfaceInfo(source);
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
	export class Request {
	    kind: string;
	    target: string;
	    port: number;
	    timeout_ms: number;
	
	    static createFrom(source: any = {}) {
	        return new Request(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.target = source["target"];
	        this.port = source["port"];
	        this.timeout_ms = source["timeout_ms"];
	    }
	}
	export class Result {
	    kind: string;
	    target: string;
	    status: string;
	    elapsed_ms: number;
	    values: string[];
	    provider: string;
	
	    static createFrom(source: any = {}) {
	        return new Result(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.target = source["target"];
	        this.status = source["status"];
	        this.elapsed_ms = source["elapsed_ms"];
	        this.values = source["values"];
	        this.provider = source["provider"];
	    }
	}

}

export namespace platform {
	
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
	export class Snapshot {
	    adapters: Adapter[];
	    counters: network.Counter[];
	
	    static createFrom(source: any = {}) {
	        return new Snapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.adapters = this.convertValues(source["adapters"], Adapter);
	        this.counters = this.convertValues(source["counters"], network.Counter);
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

}

export namespace storage {
	
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

}

export namespace tools {
	
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

}

export namespace wifi {
	
	export class Network {
	    profile: string;
	    ssid: string;
	    bssid: string;
	    signal: number;
	    authentication: number;
	    state: string;
	
	    static createFrom(source: any = {}) {
	        return new Network(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profile = source["profile"];
	        this.ssid = source["ssid"];
	        this.bssid = source["bssid"];
	        this.signal = source["signal"];
	        this.authentication = source["authentication"];
	        this.state = source["state"];
	    }
	}
	export class Secret {
	    state: string;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new Secret(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.value = source["value"];
	    }
	}

}

