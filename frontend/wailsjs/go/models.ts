export namespace settings {
	
	export class TunSettings {
	    address: string[];
	    stack: string;
	    mtu: number;
	    auto_route: boolean;
	    strict_route: boolean;
	    endpoint_independent_nat: boolean;
	    gso: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TunSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.address = source["address"];
	        this.stack = source["stack"];
	        this.mtu = source["mtu"];
	        this.auto_route = source["auto_route"];
	        this.strict_route = source["strict_route"];
	        this.endpoint_independent_nat = source["endpoint_independent_nat"];
	        this.gso = source["gso"];
	    }
	}

}

export namespace subscription {
	
	export class SubscriptionOptions {
	    user_agent?: string;
	    cookie?: string;
	    headers?: Record<string, string>;
	    preprocess?: string;
	    password?: string;
	    skip_tls?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SubscriptionOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.user_agent = source["user_agent"];
	        this.cookie = source["cookie"];
	        this.headers = source["headers"];
	        this.preprocess = source["preprocess"];
	        this.password = source["password"];
	        this.skip_tls = source["skip_tls"];
	    }
	}

}

