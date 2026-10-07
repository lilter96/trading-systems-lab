using Microsoft.EntityFrameworkCore;
using Npgsql;
using SMC.Engine.Core.Data;
using SMC.Engine.Core.Entities;
using SMC.Engine.Core.Repositories;

namespace SMC.Engine.Tests;

public class StorageIntegrationTests : IAsyncLifetime
{
    private static string ConnectionString =>
        Environment.GetEnvironmentVariable("TRADING_TEST_POSTGRES")
        ?? throw new InvalidOperationException("Set TRADING_TEST_POSTGRES to a disposable test database.");

    private readonly EngineDbContext _db;

    public StorageIntegrationTests()
    {
        var opts = new DbContextOptionsBuilder<EngineDbContext>()
            .UseNpgsql(ConnectionString)
            .Options;
        _db = new EngineDbContext(opts);
    }

    public async Task InitializeAsync()
    {
        await _db.Database.EnsureCreatedAsync();
        // Install the actual deployment invariant, not a test-only substitute.
        var deploymentSql = await File.ReadAllTextAsync(
            Path.Combine(AppContext.BaseDirectory, "deployment-init.sql"));
        var triggerStart = deploymentSql.IndexOf("CREATE OR REPLACE FUNCTION reject_audit_mutation()", StringComparison.Ordinal);
        var triggerEnd = deploymentSql.IndexOf("CREATE TABLE IF NOT EXISTS system_state", triggerStart, StringComparison.Ordinal);
        await _db.Database.ExecuteSqlRawAsync(deploymentSql[triggerStart..triggerEnd]);
    }

    public async Task DisposeAsync()
    {
        await _db.DisposeAsync();
    }

    private async Task<TradingSession> CreateTestSessionAsync()
    {
        var session = new TradingSession
        {
            StrategyName = "test_strategy",
            InitialBalance = 100_000m,
            CurrentEquity = 100_000m,
            Status = "active",
        };
        _db.TradingSessions.Add(session);
        await _db.SaveChangesAsync();
        return session;
    }

    [Fact]
    public async Task OrderLifecycle_ProposeThroughFill_RecordsCorrectStateAndAudit()
    {
        var session = await CreateTestSessionAsync();

        // propose
        var order = new Order
        {
            SessionId = session.Id,
            Asset = "BTC-USDT",
            Direction = "buy",
            Size = 1.0m,
            Status = "proposed",
            OrderType = "limit",
            LimitPrice = 60_000m,
        };
        _db.Orders.Add(order);
        _db.AuditLogEntries.Add(new AuditLogEntry
        {
            SessionId = session.Id,
            EventType = "order_proposed",
            CorrelationId = order.Id,
            Payload = $$"""{"order_id":"{{order.Id}}","price":60000,"size":1}""",
        });
        await _db.SaveChangesAsync();
        Assert.Equal("proposed", order.Status);

        // confirm
        order.Status = "confirmed";
        order.UpdatedAt = DateTimeOffset.UtcNow;
        _db.AuditLogEntries.Add(new AuditLogEntry
        {
            SessionId = session.Id,
            EventType = "order_confirmed",
            CorrelationId = order.Id,
            Payload = $$"""{"order_id":"{{order.Id}}"}""",
        });
        await _db.SaveChangesAsync();

        // send
        order.Status = "sent";
        order.UpdatedAt = DateTimeOffset.UtcNow;
        _db.AuditLogEntries.Add(new AuditLogEntry
        {
            SessionId = session.Id,
            EventType = "order_sent",
            CorrelationId = order.Id,
            Payload = $$"""{"order_id":"{{order.Id}}"}""",
        });
        await _db.SaveChangesAsync();

        // fill
        order.Status = "filled";
        order.ExecutionPrice = 60_100m;
        order.FilledAt = DateTimeOffset.UtcNow;
        order.UpdatedAt = DateTimeOffset.UtcNow;
        session.CurrentEquity -= 60_100m;
        _db.AuditLogEntries.Add(new AuditLogEntry
        {
            SessionId = session.Id,
            EventType = "order_filled",
            CorrelationId = order.Id,
            Payload = $$"""{"order_id":"{{order.Id}}","fill_price":60100}""",
        });
        await _db.SaveChangesAsync();

        // verify final state
        var loaded = await _db.Orders.AsNoTracking().FirstAsync(x => x.Id == order.Id);
        Assert.Equal("filled", loaded.Status);
        Assert.Equal(60_100m, loaded.ExecutionPrice);

        var sessionLoaded = await _db.TradingSessions.AsNoTracking().FirstAsync(x => x.Id == session.Id);
        Assert.Equal(100_000m - 60_100m, sessionLoaded.CurrentEquity);

        var auditEntries = await _db.AuditLogEntries
            .AsNoTracking()
            .Where(x => x.SessionId == session.Id)
            .OrderBy(x => x.Id)
            .ToListAsync();
        Assert.Equal(4, auditEntries.Count);
        Assert.Equal("order_proposed", auditEntries[0].EventType);
        Assert.Equal("order_confirmed", auditEntries[1].EventType);
        Assert.Equal("order_sent", auditEntries[2].EventType);
        Assert.Equal("order_filled", auditEntries[3].EventType);
    }

    [Fact]
    public async Task ConcurrentOrderCreation_NoDirtyReads()
    {
        var session = await CreateTestSessionAsync();
        var barrier = new Barrier(5);
        var errors = new List<Exception>();
        var lockObj = new object();

        void DoWork()
        {
            try
            {
                using var localDb = new EngineDbContext(
                    new DbContextOptionsBuilder<EngineDbContext>()
                        .UseNpgsql(ConnectionString)
                        .Options);
                var repo = new OrderRepository(localDb);
                var auditRepo = new AuditLogRepository(localDb);

                for (int i = 0; i < 10; i++)
                {
                    barrier.SignalAndWait();
                    var order = new Order
                    {
                        SessionId = session.Id,
                        Asset = "BTC-USDT",
                        Direction = i % 2 == 0 ? "buy" : "sell",
                        Size = 0.1m,
                        Status = "proposed",
                        OrderType = "market",
                    };
                    repo.AddAsync(order).GetAwaiter().GetResult();
                    auditRepo.InsertAsync(new AuditLogEntry
                    {
                        SessionId = session.Id,
                        EventType = "order_proposed",
                        CorrelationId = order.Id,
                        Payload = $$"""{"order_id":"{{order.Id}}"}""",
                    }).GetAwaiter().GetResult();
                    localDb.SaveChangesAsync().GetAwaiter().GetResult();
                }
            }
            catch (Exception ex)
            {
                lock (lockObj) { errors.Add(ex); }
            }
        }

        var threads = Enumerable.Range(0, 5).Select(_ => new Thread(DoWork)).ToArray();
        foreach (var t in threads) t.Start();
        foreach (var t in threads) t.Join();

        Assert.Empty(errors);

        var totalOrders = await _db.Orders.AsNoTracking()
            .Where(x => x.SessionId == session.Id)
            .CountAsync();
        Assert.Equal(50, totalOrders);

        var totalAudits = await _db.AuditLogEntries.AsNoTracking()
            .Where(x => x.SessionId == session.Id)
            .CountAsync();
        Assert.Equal(50, totalAudits);
    }

    [Fact]
    public async Task ConcurrentEquityUpdates_SerializablePreventsLostUpdate()
    {
        var session = await CreateTestSessionAsync();
        var initialEquity = session.CurrentEquity;

        async Task DeductAsync(decimal amount)
        {
            const int maxRetries = 10;
            for (int attempt = 0; attempt < maxRetries; attempt++)
            {
                try
                {
                    using var localDb = new EngineDbContext(
                        new DbContextOptionsBuilder<EngineDbContext>()
                            .UseNpgsql(ConnectionString)
                            .Options);
                    await using var tx = await localDb.Database.BeginTransactionAsync(
                        System.Data.IsolationLevel.Serializable);

                    var s = await localDb.TradingSessions.FirstAsync(x => x.Id == session.Id);
                    s.CurrentEquity -= amount;
                    await localDb.SaveChangesAsync();
                    await tx.CommitAsync();
                    return;
                }
                catch when (attempt < maxRetries - 1)
                {
                }
            }
        }

        var tasks = new List<Task>();
        for (int i = 0; i < 10; i++)
        {
            tasks.Add(DeductAsync(1000m));
        }
        await Task.WhenAll(tasks);

        var final = await _db.TradingSessions.AsNoTracking()
            .FirstAsync(x => x.Id == session.Id);
        var expected = initialEquity - (10 * 1000m);
        Assert.Equal(expected, final.CurrentEquity);
    }

    [Fact]
    public async Task AuditLog_InsertOnly_CodeLevel()
    {
        var session = await CreateTestSessionAsync();

        var entry = new AuditLogEntry
        {
            SessionId = session.Id,
            EventType = "test_event",
            Payload = "{}",
        };
        _db.AuditLogEntries.Add(entry);
        await _db.SaveChangesAsync();
        var id = entry.Id;

        // Attempt update — EF Core should not prevent it at code level,
        // but we assert no update method is used. We directly modify state.
        var loaded = await _db.AuditLogEntries.FirstAsync(x => x.Id == id);
        loaded.EventType = "modified";

        // In production the repository exposes ONLY InsertAsync.
        // But if someone directly modifies state, EF will happily save it.
        // The real enforcement is the DB trigger.
        var ex = await Record.ExceptionAsync(() => _db.SaveChangesAsync());
        Assert.NotNull(ex);
        Assert.Contains("immutable", ex.InnerException?.Message ?? ex.Message, StringComparison.OrdinalIgnoreCase);
    }

    [Fact]
    public async Task ConcurrentOrdersAndBalanceUpdates_NoDeadlocks_IntegrityPreserved()
    {
        var session = await CreateTestSessionAsync();
        const int workers = 10;
        const int opsPerWorker = 20;
        var barrier = new Barrier(workers);
        var errors = new List<Exception>();
        var lockObj = new object();

        var perOpDeductions = Enumerable.Range(0, opsPerWorker)
            .Select(i => 0.1m + i * 0.01m).ToArray();
        var expectedDeduction = perOpDeductions.Sum() * workers;

        void Worker(object? state)
        {
            var workerId = (int)state!;
            try
            {
                using var localDb = new EngineDbContext(
                    new DbContextOptionsBuilder<EngineDbContext>()
                        .UseNpgsql(ConnectionString)
                        .Options);

                for (int i = 0; i < opsPerWorker; i++)
                {
                    barrier.SignalAndWait();

                    using var tx = localDb.Database.BeginTransaction(
                        System.Data.IsolationLevel.ReadCommitted);

                    localDb.ChangeTracker.Clear();

                    var s = localDb.TradingSessions
                        .FromSqlRaw(
                            "SELECT * FROM trading_sessions WHERE id = {0} FOR UPDATE",
                            session.Id)
                        .AsTracking()
                        .First();
                    s.CurrentEquity -= perOpDeductions[i];

                    var order = new Order
                    {
                        SessionId = session.Id,
                        Asset = "BTC-USDT",
                        Direction = workerId % 2 == 0 ? "buy" : "sell",
                        Size = perOpDeductions[i],
                        Status = "proposed",
                        OrderType = "market",
                    };
                    localDb.Orders.Add(order);

                    localDb.AuditLogEntries.Add(new AuditLogEntry
                    {
                        SessionId = session.Id,
                        EventType = "order_proposed",
                        CorrelationId = order.Id,
                        Payload = $$"""{"worker":{{workerId}},"seq":{{i}}}""",
                    });

                    localDb.SaveChanges();
                    tx.Commit();
                }
            }
            catch (Exception ex)
            {
                lock (lockObj) { errors.Add(ex); }
            }
        }

        var threads = Enumerable.Range(0, workers)
            .Select(id => new Thread(Worker) { Name = $"w{id}" })
            .ToArray();
        foreach (var t in threads) t.Start(int.Parse(t.Name![1..]));
        foreach (var t in threads) t.Join();

        Assert.Empty(errors);

        var totalOrders = await _db.Orders.AsNoTracking()
            .Where(x => x.SessionId == session.Id)
            .CountAsync();
        Assert.Equal(workers * opsPerWorker, totalOrders);

        var totalAudits = await _db.AuditLogEntries.AsNoTracking()
            .Where(x => x.SessionId == session.Id)
            .CountAsync();
        Assert.Equal(workers * opsPerWorker, totalAudits);

        var final = await _db.TradingSessions.AsNoTracking()
            .FirstAsync(x => x.Id == session.Id);
        Assert.Equal(100_000m - expectedDeduction, final.CurrentEquity);
    }

    [Fact]
    public async Task AuditLog_InsertOnly_DbTriggerBlocksUpdate()
    {
        var session = await CreateTestSessionAsync();

        var entry = new AuditLogEntry
        {
            SessionId = session.Id,
            EventType = "test_event",
            Payload = "{}",
        };
        _db.AuditLogEntries.Add(entry);
        await _db.SaveChangesAsync();
        var id = entry.Id;

        using var cmd = _db.Database.GetDbConnection().CreateCommand();
        cmd.CommandText = $"UPDATE audit_log SET event_type = 'hacked' WHERE id = {id}";
        await _db.Database.OpenConnectionAsync();

        var ex = await Record.ExceptionAsync(() => cmd.ExecuteNonQueryAsync());
        Assert.NotNull(ex);
        Assert.Contains("immutable", ex.Message, StringComparison.OrdinalIgnoreCase);
    }
}
