package osquery

// go test ./pkg/osquery --run Benchmark_LogStore -bench=. -v -benchmem
// or
// -memprofile=memprofile.out

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/kolide/kit/stringutil"
	"github.com/kolide/kit/ulid"
	"github.com/kolide/launcher/ee/agent/storage"
	agentbbolt "github.com/kolide/launcher/ee/agent/storage/bbolt"
	"github.com/kolide/launcher/ee/agent/types"
	"github.com/kolide/launcher/ee/agent/types/mocks"
	"github.com/kolide/launcher/pkg/log/multislogger"
	settingsstoremock "github.com/kolide/launcher/pkg/osquery/mocks"
	"github.com/kolide/launcher/pkg/service/mock"
	"github.com/osquery/osquery-go/plugin/logger"
	"github.com/shirou/gopsutil/v3/process"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"go.etcd.io/bbolt"
)

func makeTempStores(t testing.TB, rootDirectory string) (map[storage.Store]types.KVStore, func() error) {
	closeFn := func() error { return nil }

	boltOptions := &bbolt.Options{
		Timeout:      time.Duration(30) * time.Second,
		FreelistType: bbolt.FreelistMapType,
	}

	db, err := bbolt.Open(agentbbolt.LauncherDbLocation(rootDirectory), 0600, boltOptions)
	if err != nil {
		t.Fatalf("open launcher db: %s", err)
	}
	closeFn = db.Close

	stores, err := agentbbolt.MakeStores(context.TODO(), multislogger.NewNopLogger(), db)
	if err != nil {
		t.Fatalf("failed to create stores: %s", err)
	}

	return stores, closeFn
}

func Benchmark_LogStore(b *testing.B) {
	rootDirectory := b.TempDir()
	dbFile := filepath.Join(rootDirectory, "launcher.db")
	b.Logf("Using root dir: %s\n", rootDirectory)

	stores, closeFn := makeTempStores(b, rootDirectory)
	defer closeFn()

	mockKolide := &mock.KolideService{
		PublishLogsFunc: func(ctx context.Context, nodeKey string, logType logger.LogType, logs []string) (string, string, bool, error) {
			// Do nothing, this is meant as a drain
			//b.Logf("Mock publish logs called with %d logs\n", len(logs))
			return "", "", false, nil
		},
	}

	k := mocks.NewKnapsack(b)
	k.On("Slogger").Return(multislogger.NewNopLogger()).Maybe()
	k.On("StatusLogsStore").Return(stores[storage.StatusLogsStore]).Maybe()
	k.On("ResultLogsStore").Return(stores[storage.ResultLogsStore]).Maybe()
	k.On("ConfigStore").Return(stores[storage.ConfigStore]).Maybe()
	k.On("DistributedForwardingInterval").Maybe().Return(60 * time.Second)
	k.On("RegisterChangeObserver", testifymock.Anything, testifymock.Anything).Maybe().Return()
	k.On("DeregisterChangeObserver", testifymock.Anything).Maybe().Return()

	e, err := NewExtension(context.TODO(), mockKolide, settingsstoremock.NewSettingsStoreWriter(b), k, ulid.New(), ExtensionOpts{})
	require.Nil(b, err)

	ctx := context.Background()

	// Character should be 1 byte
	// so we write a chunk of 4k
	// And if we want to write 400 megs, we need 100k iterations

	chunkSize := 1024 * 4 // 4k
	iterations := 1024 * 100

	b.ResetTimer()
	for i := 0; i < iterations; i++ {
		data := stringutil.RandomString(chunkSize)
		err := e.LogString(ctx, logger.LogTypeString, data)
		require.Nil(b, err)

		if i%1000 == 0 {
			printStats(b, dbFile, i, i*chunkSize)
		}
	}

	printStats(b, dbFile, iterations, iterations*chunkSize)
	b.Logf("Done Writing. Phew. Took %s\n", b.Elapsed())
	printStats(b, dbFile, -1, -1)
	runtime.GC()
	time.Sleep(5 * time.Second)
	runtime.GC()
	printStats(b, dbFile, -1, -2)

	b.ResetTimer()

	i := 0
	for {
		i += 1
		size, err := k.ResultLogsStore().Count()
		require.Nil(b, err)
		if size == 0 {
			break
		}

		e.writeAndPurgeLogs()

		if i%10 == 0 {
			b.Logf("Iteration %d: There are %d logs remaining\n", i, size)
			printStats(b, dbFile, i, -2)

		}
	}
	b.Logf("Done Draining. Phew. Took %s\n", b.Elapsed())
	printStats(b, dbFile, -2, -1)
	runtime.GC()
	time.Sleep(5 * time.Second)
	runtime.GC()
	printStats(b, dbFile, -2, -2)
}

func fileSizeBytes(t testing.TB, dbPath string) int64 {
	fi, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("failed to stat %s: %s", dbPath, err)
	}
	return fi.Size()
}

var runtimeStats runtime.MemStats

func printStats(t testing.TB, dbFile string, i int, charCount int) {
	runtime.ReadMemStats(&runtimeStats)

	t.Logf("Iteration %d: Wrote %d characters; db megs %.2f; heap %.2f; go %.2f; rss %.2f\n", i, charCount,
		float64(fileSizeBytes(t, dbFile))/1024.0/1024.0,
		float64(heapTotal(&runtimeStats))/1024.0/1024.0,
		float64(goMemoryUsage(&runtimeStats))/1024.0/1024.0,
		float64(rssMemory(t))/1024.0/1024.0)
}

func rssMemory(t testing.TB) uint64 {
	currentPid := os.Getpid()
	currentProcess, err := process.NewProcess(int32(currentPid))
	require.NoError(t, err)
	memInfo, err := currentProcess.MemoryInfo()
	require.NoError(t, err)
	return memInfo.RSS
}

func goMemoryUsage(m *runtime.MemStats) uint64 {
	return m.Sys - m.HeapReleased
}

func heapTotal(m *runtime.MemStats) uint64 {
	bytesAllocatedToObjects := m.HeapAlloc // both live and dead
	freeBytes := m.HeapIdle - m.HeapReleased
	unusedBytes := m.HeapInuse - m.HeapAlloc

	return bytesAllocatedToObjects + freeBytes + unusedBytes
}
