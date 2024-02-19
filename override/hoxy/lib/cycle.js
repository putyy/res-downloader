/*
 * Copyright (c) 2015 by Greg Reimer <gregreimer@gmail.com>
 * MIT License. See mit-license.txt for more info.
 */

'use strict';

var _createClass = require('babel-runtime/helpers/create-class')['default'];

var _classCallCheck = require('babel-runtime/helpers/class-call-check')['default'];

var _get = require('babel-runtime/helpers/get')['default'];

var _inherits = require('babel-runtime/helpers/inherits')['default'];

var _Promise = require('babel-runtime/core-js/promise')['default'];

var _regeneratorRuntime = require('babel-runtime/regenerator')['default'];

var _interopRequireDefault = require('babel-runtime/helpers/interop-require-default')['default'];

Object.defineProperty(exports, '__esModule', {
    value: true
});

var _request = require('./request');

var _request2 = _interopRequireDefault(_request);

var _response = require('./response');

var _response2 = _interopRequireDefault(_response);

var _streams = require('./streams');

var _streams2 = _interopRequireDefault(_streams);

var _await = require('await');

var _await2 = _interopRequireDefault(_await);

var _mkdirp = require('mkdirp');

var _mkdirp2 = _interopRequireDefault(_mkdirp);

var _lodash = require('lodash');

var _lodash2 = _interopRequireDefault(_lodash);

var _nodeStatic = require('node-static');

var _http = require('http');

var _http2 = _interopRequireDefault(_http);

var _https = require('https');

var _https2 = _interopRequireDefault(_https);

var _url = require('url');

var _url2 = _interopRequireDefault(_url);

var _fs = require('fs');

var _fs2 = _interopRequireDefault(_fs);

var _util = require('util');

var _util2 = _interopRequireDefault(_util);

var _path = require('path');

var _path2 = _interopRequireDefault(_path);

var _zlib = require('zlib');

var _zlib2 = _interopRequireDefault(_zlib);

var _events = require('events');

var _co = require('co');

var _co2 = _interopRequireDefault(_co);

var _uglyAdapter = require('ugly-adapter');

var _uglyAdapter2 = _interopRequireDefault(_uglyAdapter);

var _wait = require('./wait');

var _wait2 = _interopRequireDefault(_wait);

var _task = require('./task');

var _task2 = _interopRequireDefault(_task);

var _urlPath = require('./url-path');
const url = require("url");

var _urlPath2 = _interopRequireDefault(_urlPath);

var staticServer = (function () {

    var getStatic = (function () {
        var statics = {};
        return function (docroot) {
            var stat = statics[docroot];
            if (!stat) {
                stat = statics[docroot] = new _nodeStatic.Server(docroot);
            }
            return stat;
        };
    })();

    // Start up the server and serve out of various docroots.
    var server = _http2['default'].createServer(function (req, resp) {
        var docroot = req.headers['x-hoxy-static-docroot'];
        var pDocroot = new _urlPath2['default'](docroot);
        var stat = getStatic(pDocroot.toSystemPath());
        stat.serve(req, resp);
    }).listen(0, 'localhost');

    return server;
})();

var httpsOverHttpAgent, httpsOverHttpsAgent, httpOverHttpsAgent;
var tunnelAgent = require('tunnel-agent');
var ptGetTunnelAgent = function (requestIsSSL, externalProxyUrl) {
    var urlObject = url.parse(externalProxyUrl);
    var protocol = urlObject.protocol || 'http:';
    var port = urlObject.port;
    if (!port) {
        port = protocol === 'http:' ? 80 : 443;
    }
    var hostname = urlObject.hostname || 'localhost';

    if (requestIsSSL) {
        if (protocol === 'http:') {
            if (!httpsOverHttpAgent) {
                httpsOverHttpAgent = tunnelAgent.httpsOverHttp({
                    proxy: {
                        host: hostname,
                        port: port
                    }
                });
            }
            return httpsOverHttpAgent;
        } else {
            if (!httpsOverHttpsAgent) {
                httpsOverHttpsAgent = tunnelAgent.httpsOverHttps({
                    proxy: {
                        host: hostname,
                        port: port
                    }
                });
            }
            return httpsOverHttpsAgent;
        }
    } else {
        if (protocol === 'http:') {
            // if (!httpOverHttpAgent) {
            //     httpOverHttpAgent = tunnelAgent.httpOverHttp({
            //         proxy: {
            //             host: hostname,
            //             port: port
            //         }
            //     });
            // }
            return false;
        } else {
            if (!httpOverHttpsAgent) {
                httpOverHttpsAgent = tunnelAgent.httpOverHttps({
                    proxy: {
                        host: hostname,
                        port: port
                    }
                });
            }
            return httpOverHttpsAgent;
        }
    }
}

var ProvisionableRequest = (function () {
    function ProvisionableRequest(opts) {
        _classCallCheck(this, ProvisionableRequest);

        this._respProm = (0, _task2['default'])();
        var h = /https/i.test(opts.protocol) ? _https2['default'] : _http2['default'];
        if (opts.proxy) {
            // var proxyInfo = _url2['default'].parse(opts.proxy),
            //     proxyPort = proxyInfo.port,
            //     proxyHostname = proxyInfo.hostname,
            //     proxyPath = 'http://' + opts.hostname + (opts.port ? ':' + opts.port : '') + opts.path;
            // opts.hostname = proxyHostname;
            // opts.port = proxyPort;
            // opts.path = proxyPath;
            opts.agent = ptGetTunnelAgent(/https/i.test(opts.protocol), opts.proxy);
            // console.log('opts.agent', opts.proxy)
            opts.proxy = ''
        }

        this._writable = h.request(opts, this._respProm.resolve);
        this._writable.on('error', this._respProm.reject);
    }

    /*
     * This check() function made me scratch my head when I came back to
     * it months later. It simply does too many things. It still isn't perfect,
     * but hopefully now this beast is slightly easier to follow. It returns
     * a promise on a boolean indicating whether or not the passed file was
     * created. IF the strategy is NOT 'mirror' it resolves false since 'mirror'
     * is the only strategy that creates files. Otherwise if the file exists
     * it resolves false. Otherwise it has a side effect of creating the file
     * by requesting out to the remote server and writing the result to the
     * file, then resolves true.
     */

    _createClass(ProvisionableRequest, [{
        key: 'send',
        value: function send(readable) {
            var _this = this;

            return new _Promise(function (resolve, reject) {
                if (!readable || typeof readable === 'string') {
                    _this._writable.end(readable || '', resolve);
                } else {
                    readable.on('error', reject);
                    readable.on('end', resolve);
                    readable.pipe(_this._writable);
                }
            });
        }
    }, {
        key: 'receive',
        value: function receive() {
            return this._respProm;
        }
    }]);

    return ProvisionableRequest;
})();

function check(strategy, file, req, upstreamProxy) {
    var parsed = _url2['default'].parse(file);
    file = parsed.pathname; // stripped of query string.

    return (0, _co2['default'])(_regeneratorRuntime.mark(function callee$1$0() {
        var provReq, mirrResp, writeToFile, gunzip;
        return _regeneratorRuntime.wrap(function callee$1$0$(context$2$0) {
            while (1) switch (context$2$0.prev = context$2$0.next) {
                case 0:
                    if (!(strategy !== 'mirror')) {
                        context$2$0.next = 2;
                        break;
                    }

                    return context$2$0.abrupt('return', false);

                case 2:
                    context$2$0.prev = 2;
                    context$2$0.next = 5;
                    return (0, _uglyAdapter2['default'])(_fs2['default'].stat, file);

                case 5:
                    return context$2$0.abrupt('return', false);

                case 8:
                    context$2$0.prev = 8;
                    context$2$0.t0 = context$2$0['catch'](2);

                case 10:
                    context$2$0.next = 12;
                    return (0, _uglyAdapter2['default'])(_mkdirp2['default'], _path2['default'].dirname(file));

                case 12:
                    provReq = new ProvisionableRequest({
                        protocol: req.protocol,
                        proxy: upstreamProxy,
                        method: 'GET',
                        hostname: req.hostname,
                        port: req.port,
                        path: req.url
                    });

                    provReq.send();
                    context$2$0.next = 16;
                    return provReq.receive();

                case 16:
                    mirrResp = context$2$0.sent;

                    if (!(mirrResp.statusCode !== 200)) {
                        context$2$0.next = 19;
                        break;
                    }

                    throw new Error('mirroring failed: ' + req.fullUrl() + ' => ' + mirrResp.statusCode);

                case 19:
                    writeToFile = _fs2['default'].createWriteStream(file);

                    if (mirrResp.headers['content-encoding'] === 'gzip') {
                        gunzip = _zlib2['default'].createGunzip();

                        mirrResp = mirrResp.pipe(gunzip);
                    }
                    context$2$0.next = 23;
                    return new _Promise(function (resolve, reject) {
                        mirrResp.pipe(writeToFile);
                        mirrResp.on('end', resolve);
                        writeToFile.on('error', reject);
                    });

                case 23:
                case 'end':
                    return context$2$0.stop();
            }
        }, callee$1$0, this, [[2, 8]]);
    }));
}

// ---------------------------

var Cycle = (function (_EventEmitter) {
    _inherits(Cycle, _EventEmitter);

    function Cycle(proxy) {
        var _this2 = this;

        _classCallCheck(this, Cycle);

        _get(Object.getPrototypeOf(Cycle.prototype), 'constructor', this).call(this);
        this._proxy = proxy;
        this._request = new _request2['default']();
        this._response = new _response2['default']();
        this._request.on('log', function (log) {
            return _this2.emit('log', log);
        });
        this._response.on('log', function (log) {
            return _this2.emit('log', log);
        });
    }

    _createClass(Cycle, [{
        key: 'data',
        value: function data(name, val) {
            if (!this._userData) {
                this._userData = {};
            }
            if (arguments.length === 2) {
                this._userData[name] = val;
            }
            return this._userData[name];
        }
    }, {
        key: 'serve',
        value: function serve(opts) {

            return _co2['default'].call(this, _regeneratorRuntime.mark(function callee$2$0() {
                var req, resp, _opts, docroot, path, strategy, headers, pDocroot, pPath, pFullPath, fullSysPath,
                    created, staticResp, code, useResponse, isError, message;

                return _regeneratorRuntime.wrap(function callee$2$0$(context$3$0) {
                    while (1) switch (context$3$0.prev = context$3$0.next) {
                        case 0:
                            req = this._request;
                            resp = this._response;

                            if (typeof opts === 'string') {
                                opts = {path: opts};
                            }
                            opts = _lodash2['default'].extend({
                                docroot: _path2['default'].sep,
                                path: _url2['default'].parse(req.url).pathname,
                                strategy: 'replace'
                            }, opts);
                            _opts = opts;
                            docroot = _opts.docroot;
                            path = _opts.path;
                            strategy = _opts.strategy;
                            headers = _lodash2['default'].extend({
                                'x-hoxy-static-docroot': docroot
                            }, req.headers);

                            delete headers['if-none-match'];
                            delete headers['if-modified-since'];

                            // Now call the static file service.
                            pDocroot = new _urlPath2['default'](docroot), pPath = new _urlPath2['default'](path), pFullPath = pPath.rootTo(pDocroot), fullSysPath = pFullPath.toSystemPath();
                            context$3$0.next = 14;
                            return check(strategy, fullSysPath, req, this._proxy._upstreamProxy);

                        case 14:
                            created = context$3$0.sent;

                            if (created) {
                                this.emit('log', {
                                    level: 'info',
                                    message: 'copied ' + req.fullUrl() + ' to ' + fullSysPath
                                });
                            }
                            context$3$0.next = 18;
                            return new _Promise(function (resolve, reject) {
                                var addr = staticServer.address();
                                _http2['default'].get({
                                    hostname: addr.address,
                                    port: addr.port,
                                    headers: headers,
                                    path: pPath.toUrlPath()
                                }, resolve).on('error', reject);
                            });

                        case 18:
                            staticResp = context$3$0.sent;
                            code = staticResp.statusCode, useResponse = undefined, isError = undefined;

                            if (/^2\d\d$/.test(code)) {
                                useResponse = true;
                            } else if (/^4\d\d$/.test(code)) {
                                if (strategy === 'replace') {
                                    useResponse = true;
                                } else if (strategy === 'mirror') {
                                    isError = true;
                                }
                            } else {
                                isError = true; // nope
                            }

                            if (!isError) {
                                context$3$0.next = 26;
                                break;
                            }

                            message = _util2['default'].format('Failed to serve static file: %s => %s. Static server returned %d. Strategy: %s', req.fullUrl(), fullSysPath, staticResp.statusCode, strategy);
                            throw new Error(message);

                        case 26:
                            if (useResponse) {
                                resp._setHttpSource(staticResp);
                            }

                        case 27:
                        case 'end':
                            return context$3$0.stop();
                    }
                }, callee$2$0, this);
            }));
        }
    }, {
        key: '_setPhase',
        value: function _setPhase(phase) {
            this._phase = this._request.phase = this._response.phase = phase;
        }

        /*
         * This returns a promise on a partially fulfilled request
         * (an instance of class ProvisionableRequest). At the time
         * the promise is fulfilled, the request is in a state where
         * it's been fully piped out, but nothing received. It's up
         * to the caller of this function to call receive() on it, thus
         * getting a promise on the serverResponse object. That enables
         * hoxy to implement the 'request-sent' phase.
         */
    }, {
        key: '_sendToServer',
        value: function _sendToServer() {
            var req = this._request._finalize(),
                resp = this._response,
                upstreamProxy = this._proxy._upstreamProxy,
                source = req._source,
                pSlow = this._proxy._slow || {},
                rSlow = req.slow() || {},
                latency = rSlow.latency || 0;
            if (resp._populated) {
                return _Promise.resolve(undefined);
            }
            return _co2['default'].call(this, _regeneratorRuntime.mark(function callee$2$0() {
                var provisionableReq, brake, groupedBrake;
                return _regeneratorRuntime.wrap(function callee$2$0$(context$3$0) {
                    while (1) switch (context$3$0.prev = context$3$0.next) {
                        case 0:
                            provisionableReq = new ProvisionableRequest({
                                protocol: req.protocol,
                                proxy: upstreamProxy,
                                hostname: req.hostname,
                                port: req.port || req._getDefaultPort(),
                                method: req.method,
                                path: req.url,
                                headers: req.headers
                            });

                            if (!(latency > 0)) {
                                context$3$0.next = 4;
                                break;
                            }

                            context$3$0.next = 4;
                            return (0, _wait2['default'])(latency);

                        case 4:
                            if (rSlow.rate > 0) {
                                brake = _streams2['default'].brake(rSlow.rate);

                                source = source.pipe(brake);
                            }
                            if (pSlow.rate) {
                                groupedBrake = pSlow.rate.throttle();

                                source = source.pipe(groupedBrake);
                            }
                            if (pSlow.up) {
                                groupedBrake = pSlow.up.throttle();

                                source = source.pipe(groupedBrake);
                            }
                            req._tees().forEach(function (writable) {
                                return source.pipe(writable);
                            });
                            context$3$0.next = 10;
                            return provisionableReq.send(source);

                        case 10:
                            return context$3$0.abrupt('return', provisionableReq);

                        case 11:
                        case 'end':
                            return context$3$0.stop();
                    }
                }, callee$2$0, this);
            }));
        }
    }, {
        key: '_sendToClient',
        value: function _sendToClient(outResp) {
            var resp = this._response._finalize(),
                source = resp._source,
                rSlow = resp.slow() || {},
                pSlow = this._proxy._slow || {},
                rLatency = rSlow.latency || 0,
                pLatency = pSlow.latency || 0,
                latency = Math.max(pLatency, rLatency);
            return _co2['default'].call(this, _regeneratorRuntime.mark(function callee$2$0() {
                var brake, groupedBrake, tees;
                return _regeneratorRuntime.wrap(function callee$2$0$(context$3$0) {
                    while (1) switch (context$3$0.prev = context$3$0.next) {
                        case 0:
                            if (!(latency > 0)) {
                                context$3$0.next = 3;
                                break;
                            }

                            context$3$0.next = 3;
                            return (0, _wait2['default'])(latency);

                        case 3:
                            outResp.writeHead(resp.statusCode, resp.headers);
                            if (rSlow.rate > 0) {
                                brake = _streams2['default'].brake(rSlow.rate);

                                source = source.pipe(brake);
                            }
                            if (pSlow.rate) {
                                groupedBrake = pSlow.rate.throttle();

                                source = source.pipe(groupedBrake);
                            }
                            if (pSlow.down) {
                                groupedBrake = pSlow.down.throttle();

                                source = source.pipe(groupedBrake);
                            }
                            tees = resp._tees();

                            tees.forEach(function (writable) {
                                return source.pipe(writable);
                            });
                            context$3$0.next = 11;
                            return new _Promise(function (resolve, reject) {
                                source.on('error', reject);
                                source.on('end', resolve);
                                source.pipe(outResp);
                            });

                        case 11:
                        case 'end':
                            return context$3$0.stop();
                    }
                }, callee$2$0, this);
            }));
        }
    }, {
        key: '_start',
        value: function _start() {
            // for now, an immediately-kept promise
            return (0, _await2['default'])('started').keep('started');
        }
    }]);

    return Cycle;
})(_events.EventEmitter);

exports['default'] = Cycle;
module.exports = exports['default'];

// file does not exist, so continue

// TODO: test coverage for mkdirp

// TODO: test coverage

// First, get all our ducks in a row WRT to
// options, setting variables, etc.
// return the outer promise
// wait for it all to pipe out