// Runs each step the worker hands it, in the worker's build directory. From bb-deployments'
// bare/config/runner.jsonnet (7cd419f), with the diagnostics port on localhost.
local common = import 'common.libsonnet';

{
  buildDirectoryPath: std.extVar('PWD') + '/worker/build',
  global: common.globalWithDiagnosticsHttpServer('127.0.0.1:9987'),
  grpcServers: [{
    listenPaths: ['worker/runner'],
    authenticationPolicy: { allow: {} },
  }],
}
