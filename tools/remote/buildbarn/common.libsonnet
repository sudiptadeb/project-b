// Shared by every Buildbarn process. From bb-deployments' bare/config (7cd419f, 2026-11-06);
// only the message size differs: Chromium's compile steps carry large input trees.
{
  globalWithDiagnosticsHttpServer(listenAddress): {
    diagnosticsHttpServer: {
      httpServers: [{
        listenAddresses: [listenAddress],
        authenticationPolicy: { allow: {} },
      }],
      enablePrometheus: true,
      enablePprof: true,
      enableActiveSpans: true,
    },
  },
  maximumMessageSizeBytes: 16 * 1024 * 1024,
}
