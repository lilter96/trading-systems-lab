using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata.Builders;
using SMC.Engine.Core.Entities;

namespace SMC.Engine.Core.Data.EntityConfigurations;

public class SystemStateConfiguration : IEntityTypeConfiguration<SystemState>
{
    public void Configure(EntityTypeBuilder<SystemState> builder)
    {
        builder.ToTable("system_state");
        builder.HasKey(x => x.Id);
        builder.Property(x => x.Id).HasColumnName("id");
        builder.Property(x => x.SystemId).HasColumnName("system_id").HasMaxLength(50).IsRequired();
        builder.Property(x => x.State).HasColumnName("state").HasMaxLength(20).IsRequired();
        builder.Property(x => x.TradingEnabled).HasColumnName("trading_enabled");
        builder.Property(x => x.UpdatedAt).HasColumnName("updated_at");
        builder.HasIndex(x => x.SystemId).IsUnique();
    }
}
