using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata.Builders;
using SMC.Engine.Core.Entities;

namespace SMC.Engine.Core.Data.EntityConfigurations;

public class RiskLimitConfiguration : IEntityTypeConfiguration<RiskLimit>
{
    public void Configure(EntityTypeBuilder<RiskLimit> builder)
    {
        builder.ToTable("risk_limits");
        builder.HasKey(x => x.Id);
        builder.Property(x => x.Id).HasColumnName("id");
        builder.Property(x => x.SessionId).HasColumnName("session_id");
        builder.Property(x => x.DailyLossLimit).HasColumnName("daily_loss_limit").HasColumnType("decimal(18,8)");
        builder.Property(x => x.MaxOpenPositions).HasColumnName("max_open_positions");
        builder.Property(x => x.GlobalHalt).HasColumnName("global_halt");
        builder.Property(x => x.MaxPositionSize).HasColumnName("max_position_size").HasColumnType("decimal(18,8)");
        builder.Property(x => x.MaxLeverage).HasColumnName("max_leverage").HasColumnType("decimal(10,2)");
        builder.Property(x => x.UpdatedAt).HasColumnName("updated_at");

        builder.HasOne(x => x.Session)
            .WithMany()
            .HasForeignKey(x => x.SessionId)
            .OnDelete(DeleteBehavior.SetNull);
    }
}
