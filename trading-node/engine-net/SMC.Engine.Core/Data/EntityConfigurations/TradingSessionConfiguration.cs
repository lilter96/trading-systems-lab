using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata.Builders;
using SMC.Engine.Core.Entities;

namespace SMC.Engine.Core.Data.EntityConfigurations;

public class TradingSessionConfiguration : IEntityTypeConfiguration<TradingSession>
{
    public void Configure(EntityTypeBuilder<TradingSession> builder)
    {
        builder.ToTable("trading_sessions");
        builder.HasKey(x => x.Id);
        builder.Property(x => x.Id).HasColumnName("id");
        builder.Property(x => x.StrategyName).HasColumnName("strategy_name").HasMaxLength(100).IsRequired();
        builder.Property(x => x.StartedAt).HasColumnName("started_at");
        builder.Property(x => x.EndedAt).HasColumnName("ended_at");
        builder.Property(x => x.InitialBalance).HasColumnName("initial_balance").HasColumnType("decimal(18,8)");
        builder.Property(x => x.CurrentEquity).HasColumnName("current_equity").HasColumnType("decimal(18,8)");
        builder.Property(x => x.Status).HasColumnName("status").HasMaxLength(20).IsRequired();
        builder.Property(x => x.CreatedAt).HasColumnName("created_at");
    }
}
