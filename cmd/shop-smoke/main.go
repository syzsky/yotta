package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/yottaapp/yotta/internal/automation/browsercdp"
)

func main() {
	endpoint := flag.String("endpoint", "http://127.0.0.1:9237", "Yotta WebView CDP endpoint")
	action := flag.String("action", "start-publish", "start-publish or status")
	listen := flag.String("listen", "127.0.0.1:8092", "authorization capture proxy address")
	target := flag.String("target", "http://127.0.0.1:3011", "Account origin")
	screenshot := flag.String("screenshot", "", "market screenshot output")
	workflowID := flag.String("workflow-id", "", "installed workflow to open")
	flag.Parse()
	if *action == "proxy" {
		fatal(http.ListenAndServe(*listen, captureProxy(*target)))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	targets, err := browsercdp.NewService(*endpoint).ListTargets(ctx, *endpoint)
	if err != nil || len(targets) != 1 {
		fatal(fmt.Errorf("resolve Yotta WebView: targets=%d: %w", len(targets), err))
	}
	client, err := browsercdp.DialWebSocketClient(ctx, targets[0].WebSocketDebuggerURL)
	if err != nil {
		fatal(err)
	}
	defer client.Close()
	expression := `JSON.stringify({state:globalThis.__yottaShopSmoke?.state||"idle",result:globalThis.__yottaShopSmoke?.result||null,error:globalThis.__yottaShopSmoke?.error||null})`
	if *action == "start-publish" {
		expression = `(async()=>{globalThis.__yottaShopSmoke={state:"starting"};try{const api=await import('/bindings/github.com/yottaapp/yotta/internal/services/workflow/service.ts');const source=await api.CreateSourceWithMetadata({name:"Yotta Shop Smoke",description:"真实市场发布、搜索和安装验收工作流",category:"示例",tags:["shop","smoke"]});globalThis.__yottaShopSmoke.state="authorizing";await api.LoginRegistry();const release=await api.PublishSourceToRegistry({workflowId:source.workflowId,releaseVersion:"1.0.0",title:"Yotta Shop Smoke",summary:"真实 Registry 市场端到端验收",releaseNotes:"首次可安装版本",examples:[]});const result={...release,sourceWorkflowId:source.workflowId};sessionStorage.setItem('yotta.shopSmoke.result',JSON.stringify(result));globalThis.__yottaShopSmoke={state:"published",result};}catch(error){globalThis.__yottaShopSmoke={state:"failed",error:String(error?.message||error)}}})();"started"`
	}
	if *action == "reload-local" {
		expression = `(()=>{location.hash='#/workflows';location.reload();return "reloading"})()`
	}
	if *action == "market" {
		expression = `(async()=>{const sleep=ms=>new Promise(r=>setTimeout(r,ms));let market=[...document.querySelectorAll('button')].find(b=>b.textContent?.includes('在线市场'));if(!market){location.hash='#/workflows';for(let i=0;i<100&&!market;i++){await sleep(100);market=[...document.querySelectorAll('button')].find(b=>b.textContent?.includes('在线市场'));}}if(!market)throw new Error('online market tab not found');market.click();for(let i=0;i<100;i++){if(document.querySelector('[data-testid="workflow-market"]')?.textContent?.includes('Yotta Shop Smoke'))break;await sleep(100);}const panel=document.querySelector('[data-testid="workflow-market"]');if(!panel?.textContent?.includes('Yotta Shop Smoke'))throw new Error('published workflow not found');return JSON.stringify({href:location.href,text:panel.textContent});})()`
	}
	if *action == "install" {
		expression = `(async()=>{const sleep=ms=>new Promise(r=>setTimeout(r,ms));const button=document.querySelector('[data-testid="market-install"], [data-testid="market-open"]')||[...document.querySelectorAll('button')].find(b=>b.textContent?.includes('安装并打开'));if(!button)throw new Error('install action not found');button.click();let opened=button.dataset.testid==='market-open';for(let i=0;i<150;i++){const open=document.querySelector('[data-testid="market-open"]');if(!opened&&open&&!open.disabled&&!document.querySelector('[data-testid="market-install"]')){open.click();opened=true;}if(location.hash.includes('/edit'))return JSON.stringify({href:location.href});await sleep(100);}throw new Error('installed workflow did not open');})()`
	}
	if *action == "open-installed" {
		encoded, _ := json.Marshal(*workflowID)
		expression = fmt.Sprintf(`(async()=>{const sleep=ms=>new Promise(r=>setTimeout(r,ms));location.hash='#/workflows/'+%s+'/edit';for(let i=0;i<150;i++){if(document.querySelector('[data-testid="workflow-canvas"]'))return JSON.stringify({href:location.href,title:document.body.textContent});await sleep(100);}throw new Error('installed workflow editor did not open');})()`, encoded)
	}
	if *action == "installed" {
		expression = `(()=>{const canvas=document.querySelector('[data-testid="workflow-canvas"]');if(!location.hash.includes('/edit')||!canvas||!document.body.textContent?.includes('Yotta Shop Smoke'))throw new Error('installed workflow is not open');return JSON.stringify({href:location.href,title:'Yotta Shop Smoke'});})()`
	}
	if *action == "ui-publish" {
		expression = `(async()=>{const sleep=ms=>new Promise(r=>setTimeout(r,ms));const result=JSON.parse(sessionStorage.getItem('yotta.shopSmoke.result')||'null');const workflowId=result?.sourceWorkflowId;if(!workflowId)throw new Error('published source identity is missing');const suffix=Date.now()%100000;const summary='通过 Yotta 发布界面的真实端到端版本 '+suffix;let row;for(let i=0;i<100&&!row;i++){await sleep(100);row=document.querySelector('[data-testid="workflow-library-row"][data-workflow-id="'+CSS.escape(workflowId)+'"]');}if(!row)throw new Error('publish source row not found');row.querySelector('[data-testid="workflow-row-menu"]')?.click();let publish;for(let i=0;i<50&&!publish;i++){await sleep(100);publish=[...document.querySelectorAll('button')].find(b=>b.textContent?.includes('发布到市场'));}if(!publish)throw new Error('publish menu action not found');publish.click();for(let i=0;i<50&&!document.querySelector('[data-testid="workflow-publish-submit"]');i++)await sleep(100);const version=document.querySelector('[data-testid="workflow-version-0"]');const summaryInput=document.querySelector('[data-testid="workflow-publish-summary"]');const set=(element,value)=>{const setter=Object.getOwnPropertyDescriptor(element instanceof HTMLTextAreaElement?HTMLTextAreaElement.prototype:HTMLInputElement.prototype,'value').set;setter.call(element,value);element.dispatchEvent(new Event('input',{bubbles:true}));};set(version,'1');set(document.querySelector('[data-testid="workflow-version-1"]'),'0');set(document.querySelector('[data-testid="workflow-version-2"]'),String(suffix));set(summaryInput,summary);document.querySelector('[data-testid="workflow-publish-submit"]').click();for(let i=0;i<150;i++){if(document.querySelector('[data-testid="workflow-market"]')?.textContent?.includes(summary))return JSON.stringify({href:location.href,summary});await sleep(100);}throw new Error('UI publication did not appear in market');})()`
	}
	awaitPromise := *action == "market" || *action == "install" || *action == "open-installed" || *action == "ui-publish"
	result, err := client.Call(ctx, "Runtime.evaluate", map[string]any{"expression": expression, "returnByValue": true, "awaitPromise": awaitPromise})
	if err != nil {
		fatal(err)
	}
	if details, failed := result["exceptionDetails"]; failed {
		fatal(fmt.Errorf("WebView evaluation failed: %v", details))
	}
	raw, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(raw))
	if *screenshot != "" {
		if _, err := client.Call(ctx, "Page.enable", nil); err != nil {
			fatal(err)
		}
		captured, err := client.Call(ctx, "Page.captureScreenshot", map[string]any{"format": "png", "fromSurface": true})
		if err != nil {
			fatal(err)
		}
		data, _ := captured["data"].(string)
		decoded, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(*screenshot), 0o755); err != nil {
			fatal(err)
		}
		if err := os.WriteFile(*screenshot, decoded, 0o600); err != nil {
			fatal(err)
		}
	}
}

func captureProxy(target string) http.Handler {
	var mu sync.RWMutex
	latest := ""
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("GET /latest", func(w http.ResponseWriter, _ *http.Request) {
		mu.RLock()
		defer mu.RUnlock()
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, latest)
	})
	mux.HandleFunc("GET /oauth2/authorize", func(w http.ResponseWriter, r *http.Request) {
		location := strings.TrimRight(target, "/") + "/oauth2/authorize?" + r.URL.RawQuery
		mu.Lock()
		latest = location
		mu.Unlock()
		http.Redirect(w, r, location, http.StatusFound)
	})
	return mux
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
