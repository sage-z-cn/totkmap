export namespace main {
	
	export class Config {
	    saveDirRyujinx?: string;
	    saveDirEden?: string;
	    emu?: string;
	    tls?: boolean;
	    autoOpenMap?: boolean;
	    autoSwitchToMap?: boolean;
	    autoMoveToSecondary?: boolean;
	    autoMoveTarget?: string;
	    autoStartOnGame?: boolean;
	    gameExe?: string;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.saveDirRyujinx = source["saveDirRyujinx"];
	        this.saveDirEden = source["saveDirEden"];
	        this.emu = source["emu"];
	        this.tls = source["tls"];
	        this.autoOpenMap = source["autoOpenMap"];
	        this.autoSwitchToMap = source["autoSwitchToMap"];
	        this.autoMoveToSecondary = source["autoMoveToSecondary"];
	        this.autoMoveTarget = source["autoMoveTarget"];
	        this.autoStartOnGame = source["autoStartOnGame"];
	        this.gameExe = source["gameExe"];
	    }
	}

}

