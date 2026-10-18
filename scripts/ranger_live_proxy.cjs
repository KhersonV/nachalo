// Local test entrypoint: workspace frontend + real, already-running services.
// No Docker or nginx configuration is changed; no responses are mocked.
const http = require('node:http');
const httpProxy = require('next/dist/compiled/http-proxy');
const proxy = httpProxy.createProxyServer({ ws: true, changeOrigin: true });
const backend = path => /^\/(auth|matchmaking|api|create|healthz)\//.test(path) ||
  path.startsWith('/ws') || (path.startsWith('/game/') && !/^\/game\/stats(?:[/?]|$)/.test(path));
const target = url => backend(url.split('?')[0]) ? 'http://127.0.0.1:80' : 'http://127.0.0.1:3002';
proxy.on('error', (_, req, res) => { if (res?.writeHead) res.writeHead(502).end('Local upstream unavailable'); });
const server = http.createServer((req,res) => proxy.web(req,res,{target: target(req.url)}));
server.on('upgrade',(req,socket,head) => proxy.ws(req,socket,head,{target: target(req.url)}));
server.listen(3003,'127.0.0.1',() => console.log('Ranger live test: http://localhost:3003 (real services through localhost:80)'));
