// From bb-deployments' bare/config/scheduler.jsonnet (7cd419f, 2026-11-06). Every port is on localhost.
local common = import 'common.libsonnet';
local topology = import 'topology.libsonnet';

{
  clientGrpcServers: [{
    listenAddresses: ['127.0.0.1:8982'],
    authenticationPolicy: { allow: {} },
  }],
  workerGrpcServers: [{
    listenAddresses: ['127.0.0.1:8983'],
    authenticationPolicy: { allow: {} },
  }],
  buildQueueStateGrpcServers: [{
    listenAddresses: ['127.0.0.1:8984'],
    authenticationPolicy: { allow: {} },
  }],
  contentAddressableStorage: {
    chunkStorage: topology.chunkStorage,
    chunkMappingStorage: topology.chunkMappingStorage,
    cdcParameterCache: topology.cdcParameterCache,
  },
  maximumMessageSizeBytes: common.maximumMessageSizeBytes,
  global: common.globalWithDiagnosticsHttpServer('127.0.0.1:9982'),
  executeAuthorizer: { allow: {} },
  modifyDrainsAuthorizer: { allow: {} },
  killOperationsAuthorizer: { allow: {} },
  synchronizeAuthorizer: { allow: {} },
  actionRouter: {
    simple: {
      platformKeyExtractor: { action: {} },
      invocationKeyExtractors: [
        { correlatedInvocationsId: {} },
        { toolInvocationId: {} },
      ],
      initialSizeClassAnalyzer: {
        defaultExecutionTimeout: '1800s',
        maximumExecutionTimeout: '7200s',
      },
    },
  },
  platformQueueWithNoWorkersTimeout: '900s',
}
