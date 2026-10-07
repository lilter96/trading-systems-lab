using Microsoft.EntityFrameworkCore;
using SMC.Engine.Core.Data.EntityConfigurations;
using SMC.Engine.Core.Entities;

namespace SMC.Engine.Core.Data;

public class EngineDbContext : DbContext
{
    public EngineDbContext(DbContextOptions<EngineDbContext> options) : base(options)
    {
    }

    public DbSet<TradingSession> TradingSessions => Set<TradingSession>();
    public DbSet<Order> Orders => Set<Order>();
    public DbSet<Position> Positions => Set<Position>();
    public DbSet<RiskLimit> RiskLimits => Set<RiskLimit>();
    public DbSet<StrategyConfig> StrategyConfigs => Set<StrategyConfig>();
    public DbSet<AuditLogEntry> AuditLogEntries => Set<AuditLogEntry>();
    public DbSet<SystemState> SystemStates => Set<SystemState>();

    protected override void OnModelCreating(ModelBuilder modelBuilder)
    {
        modelBuilder.ApplyConfiguration(new TradingSessionConfiguration());
        modelBuilder.ApplyConfiguration(new OrderConfiguration());
        modelBuilder.ApplyConfiguration(new PositionConfiguration());
        modelBuilder.ApplyConfiguration(new RiskLimitConfiguration());
        modelBuilder.ApplyConfiguration(new StrategyConfigConfiguration());
        modelBuilder.ApplyConfiguration(new AuditLogEntryConfiguration());
        modelBuilder.ApplyConfiguration(new SystemStateConfiguration());
    }
}
