export namespace main {
	
	export class AgentInfo {
	    name: string;
	    model: string;
	
	    static createFrom(source: any = {}) {
	        return new AgentInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.model = source["model"];
	    }
	}
	export class SafeWorkingConfig {
	    quietHoursEnabled: boolean;
	    quietHoursStart: string;
	    quietHoursEnd: string;
	    breakEnabled: boolean;
	    workMinutes: number;
	    breakMinutes: number;
	
	    static createFrom(source: any = {}) {
	        return new SafeWorkingConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.quietHoursEnabled = source["quietHoursEnabled"];
	        this.quietHoursStart = source["quietHoursStart"];
	        this.quietHoursEnd = source["quietHoursEnd"];
	        this.breakEnabled = source["breakEnabled"];
	        this.workMinutes = source["workMinutes"];
	        this.breakMinutes = source["breakMinutes"];
	    }
	}
	export class AppConfig {
	    slackToken: string;
	    slackOwnerId: string;
	    githubRepos: string[];
	    safeWorking: SafeWorkingConfig;
	    apiPort: number;
	    apiKey: string;
	    apiEnabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AppConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.slackToken = source["slackToken"];
	        this.slackOwnerId = source["slackOwnerId"];
	        this.githubRepos = source["githubRepos"];
	        this.safeWorking = this.convertValues(source["safeWorking"], SafeWorkingConfig);
	        this.apiPort = source["apiPort"];
	        this.apiKey = source["apiKey"];
	        this.apiEnabled = source["apiEnabled"];
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
	export class CommandInfo {
	    name: string;
	    source: string;
	    type: string;
	    description: string;
	
	    static createFrom(source: any = {}) {
	        return new CommandInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.source = source["source"];
	        this.type = source["type"];
	        this.description = source["description"];
	    }
	}
	export class ContextInfo {
	    usedPercentage: number;
	    remainingPercentage: number;
	    model: string;
	
	    static createFrom(source: any = {}) {
	        return new ContextInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.usedPercentage = source["usedPercentage"];
	        this.remainingPercentage = source["remainingPercentage"];
	        this.model = source["model"];
	    }
	}
	export class CreateSessionOpts {
	    name: string;
	    dir: string;
	    cols: number;
	    rows: number;
	    resume: boolean;
	    claudeSessionId: string;
	
	    static createFrom(source: any = {}) {
	        return new CreateSessionOpts(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.dir = source["dir"];
	        this.cols = source["cols"];
	        this.rows = source["rows"];
	        this.resume = source["resume"];
	        this.claudeSessionId = source["claudeSessionId"];
	    }
	}
	export class FileChange {
	    path: string;
	    status: string;
	    staged: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FileChange(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.status = source["status"];
	        this.staged = source["staged"];
	    }
	}
	export class FileContentData {
	    content: string;
	    language: string;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new FileContentData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.content = source["content"];
	        this.language = source["language"];
	        this.path = source["path"];
	    }
	}
	export class FileDiffData {
	    oldFileName: string;
	    oldContent: string;
	    newFileName: string;
	    newContent: string;
	    hunks: string;
	    language: string;
	    additions: number;
	    deletions: number;
	
	    static createFrom(source: any = {}) {
	        return new FileDiffData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.oldFileName = source["oldFileName"];
	        this.oldContent = source["oldContent"];
	        this.newFileName = source["newFileName"];
	        this.newContent = source["newContent"];
	        this.hunks = source["hunks"];
	        this.language = source["language"];
	        this.additions = source["additions"];
	        this.deletions = source["deletions"];
	    }
	}
	export class GitHubNotification {
	    id: string;
	    title: string;
	    type: string;
	    reason: string;
	    repo: string;
	    url: string;
	    unread: boolean;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new GitHubNotification(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.type = source["type"];
	        this.reason = source["reason"];
	        this.repo = source["repo"];
	        this.url = source["url"];
	        this.unread = source["unread"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class GitHubPR {
	    repo: string;
	    number: number;
	    title: string;
	    author: string;
	    reviewDecision: string;
	    url: string;
	
	    static createFrom(source: any = {}) {
	        return new GitHubPR(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.repo = source["repo"];
	        this.number = source["number"];
	        this.title = source["title"];
	        this.author = source["author"];
	        this.reviewDecision = source["reviewDecision"];
	        this.url = source["url"];
	    }
	}
	export class MCPServer {
	    name: string;
	    command: string;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new MCPServer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.command = source["command"];
	        this.status = source["status"];
	    }
	}
	export class ProcessInfo {
	    pid: number;
	    command: string;
	    fullCmd: string;
	    type: string;
	    elapsed: string;
	    elapsedMs: number;
	    children: number;
	
	    static createFrom(source: any = {}) {
	        return new ProcessInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pid = source["pid"];
	        this.command = source["command"];
	        this.fullCmd = source["fullCmd"];
	        this.type = source["type"];
	        this.elapsed = source["elapsed"];
	        this.elapsedMs = source["elapsedMs"];
	        this.children = source["children"];
	    }
	}
	
	export class SavedSessionHistory {
	    name: string;
	    directory: string;
	    createdAt: number;
	    closedAt: number;
	    lastMessage?: string;
	
	    static createFrom(source: any = {}) {
	        return new SavedSessionHistory(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.directory = source["directory"];
	        this.createdAt = source["createdAt"];
	        this.closedAt = source["closedAt"];
	        this.lastMessage = source["lastMessage"];
	    }
	}
	export class SavedSessionTab {
	    id: string;
	    name: string;
	    directory: string;
	    createdAt: number;
	
	    static createFrom(source: any = {}) {
	        return new SavedSessionTab(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.directory = source["directory"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class SessionInfo {
	    id: string;
	    slug: string;
	    name: string;
	    dir: string;
	
	    static createFrom(source: any = {}) {
	        return new SessionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.slug = source["slug"];
	        this.name = source["name"];
	        this.dir = source["dir"];
	    }
	}
	export class SessionRecord {
	    slug: string;
	    name: string;
	    directory: string;
	    claudeSessionId?: string;
	    createdAt: number;
	    closedAt?: number;
	    status: string;
	    lastMsg?: string;
	
	    static createFrom(source: any = {}) {
	        return new SessionRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.slug = source["slug"];
	        this.name = source["name"];
	        this.directory = source["directory"];
	        this.claudeSessionId = source["claudeSessionId"];
	        this.createdAt = source["createdAt"];
	        this.closedAt = source["closedAt"];
	        this.status = source["status"];
	        this.lastMsg = source["lastMsg"];
	    }
	}
	export class SessionsData {
	    sessions: SessionRecord[];
	    recentDirs: string[];
	    tabs?: SavedSessionTab[];
	    history?: SavedSessionHistory[];
	
	    static createFrom(source: any = {}) {
	        return new SessionsData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessions = this.convertValues(source["sessions"], SessionRecord);
	        this.recentDirs = source["recentDirs"];
	        this.tabs = this.convertValues(source["tabs"], SavedSessionTab);
	        this.history = this.convertValues(source["history"], SavedSessionHistory);
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
	export class UpdateInfo {
	    available: boolean;
	    version: string;
	    currentVersion: string;
	    url: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.version = source["version"];
	        this.currentVersion = source["currentVersion"];
	        this.url = source["url"];
	    }
	}

}

