using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata.Builders;
using SMC.Engine.Core.Entities;

namespace SMC.Engine.Core.Data.EntityConfigurations;

public class PositionConfiguration : IEntityTypeConfiguration<Position>
{
    public void Configure(EntityTypeBuilder<Position> builder)
    {
        builder.ToTable("positions");
        builder.HasKey(x => x.Id);
        builder.Property(x => x.Id).HasColumnName("id");
        builder.Property(x => x.SessionId).HasColumnName("session_id");
        builder.Property(x => x.Asset).HasColumnName("asset").HasMaxLength(20).IsRequired();
        builder.Property(x => x.Direction).HasColumnName("direction").HasMaxLength(10).IsRequired();
        builder.Property(x => x.EntryPrice).HasColumnName("entry_price").HasColumnType("decimal(18,8)");
        builder.Property(x => x.CurrentSize).HasColumnName("current_size").HasColumnType("decimal(18,8)");
        builder.Property(x => x.StopLoss).HasColumnName("stop_loss").HasColumnType("decimal(18,8)");
        builder.Property(x => x.TakeProfit).HasColumnName("take_profit").HasColumnType("decimal(18,8)");
        builder.Property(x => x.Status).HasColumnName("status").HasMaxLength(20).IsRequired();
        builder.Property(x => x.OpenedAt).HasColumnName("opened_at");
        builder.Property(x => x.ClosedAt).HasColumnName("closed_at");
        builder.Property(x => x.RealizedPnl).HasColumnName("realized_pnl").HasColumnType("decimal(18,8)");

        builder.HasOne(x => x.Session)
            .WithMany(s => s.Positions)
            .HasForeignKey(x => x.SessionId)
            .OnDelete(DeleteBehavior.Restrict);
    }
}
