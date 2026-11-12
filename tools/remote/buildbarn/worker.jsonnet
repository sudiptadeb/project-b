// The one worker: native macOS, running compile steps straight on the Mac Studio. Based on
// bb-deployments' bare/config/worker.jsonnet (7cd419f), sized for Chromium.
//
// It serves one platform, OSFamily=Darwin, which tools/remote/siso/main.star gives every
// compile step it sends.
local common = import 'common.libsonnet';
local topology = import 'topology.libsonnet';

local slots = std.parseInt(std.extVar('PB_RBE_SLOTS'));

{
  blobstore: {
    actionCache: topology.actionCache,
    contentAddressableStorage: {
      chunkStorage: {
        readCaching: {
          slow: topology.chunkStorage,
          fast: {
            'local': {
              keyLocationMap: {
                onBlockDevice: {
                  file: {
                    path: 'worker/cs/key_location_map',
                    sizeBytes: 400 * 1024 * 1024,
                  },
                },
                maximumGetAttempts: 16,
                maximumPutAttempts: 64,
              },
              oldBlocks: 8,
              currentBlocks: 24,
              newBlocks: 3,
              blocksOnBlockDevice: {
                source: {
                  file: {
                    path: 'worker/cs/blocks',
                    sizeBytes: 16 * 1024 * 1024 * 1024,
                  },
                },
                spareBlocks: 3,
              },
              persistent: {
                stateDirectoryPath: 'worker/cs/persistent_state',
                minimumEpochInterval: '300s',
              },
            },
          },
          replicator: { deduplicating: { 'local': {} } },
        },
      },
      chunkMappingStorage: topology.chunkMappingStorage,
      cdcParameterCache: topology.cdcParameterCache,
      // The virtual build directory reads the same blobs again and again, so keep parsed
      // chunk mappings and decompressed chunks in memory, as the sample does for it.
      chunkMappingCache: {
        cacheSize: 100000,
        cacheDuration: '600s',
        cacheReplacementPolicy: 'LEAST_RECENTLY_USED',
      },
      chunkCache: {
        cacheSize: 100000,
        cacheDuration: '600s',
        cacheReplacementPolicy: 'LEAST_RECENTLY_USED',
      },
    },
  },
  maximumMessageSizeBytes: common.maximumMessageSizeBytes,
  scheduler: { address: 'localhost:8983' },
  global: common.globalWithDiagnosticsHttpServer('127.0.0.1:9986'),
  buildDirectories: [{
    // A virtual build directory: the worker serves each step's inputs over its own NFSv4
    // server, mounted with macOS's NFS client, and fetches a file only when clang reads
    // it. The native kind hardlinks every input into place first; a Chromium compile has
    // thousands, so 30 steps at a time kept the Mac busy in the kernel and barely
    // compiling. Supported on macOS 15 and later.
    virtual: {
      mount: {
        mountPath: std.extVar('PWD') + '/worker/build',
        nfsv4: {
          darwin: { socketPath: std.extVar('PWD') + '/worker/nfsv4/socket' },
          enforcedLeaseTime: '120s',
          announcedLeaseTime: '60s',
        },
      },
      maximumExecutionTimeoutCompensation: '3600s',
      maximumWritableFileUploadDelay: '60s',
    },
    runners: [{
      endpoint: { address: 'unix:worker/runner' },
      concurrency: slots,
      maximumFilePoolFileCount: 100000,
      maximumFilePoolSizeBytes: 8 * 1024 * 1024 * 1024,
      platform: { properties: [{ name: 'OSFamily', value: 'Darwin' }] },
      workerId: { hostname: std.extVar('PB_RBE_HOST') },
    }],
  }],
  filePool: {
    blockDevice: {
      file: {
        path: 'worker/filepool',
        sizeBytes: 8 * 1024 * 1024 * 1024,
      },
    },
  },
  inputDownloadConcurrency: 64,
  outputUploadConcurrency: 64,
  directoryCache: {
    maximumCount: 100000,
    maximumSizeBytes: 256 * 1024 * 1024,
    cacheReplacementPolicy: 'LEAST_RECENTLY_USED',
  },
}
