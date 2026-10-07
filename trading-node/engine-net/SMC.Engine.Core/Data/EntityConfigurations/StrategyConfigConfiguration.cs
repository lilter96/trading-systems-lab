using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata.Builders;
using SMC.Engine.Core.Entities;

namespace SMC.Engine.Core.Data.EntityConfigurations;

public class StrategyConfigConfiguration : IEntityTypeConfiguration<StrategyConfig>
{
    public void Configure(EntityTypeBuilder<StrategyConfig> builder)
    {
        builder.ToTable("strategy_config");
        builder.HasKey(x => x.Id);
        builder.Property(x => x.Id).HasColumnName("id");
        builder.Property(x => x.Name).HasColumnName("name").HasMaxLength(100).IsRequired();
        builder.HasIndex(x => x.Name).IsUnique();
        builder.Property(x => x.Value).HasColumnName("value").HasColumnType("jsonb").IsRequired();
        builder.Property(x => x.Description).HasColumnName("description");
        builder.Property(x => x.UpdatedAt).HasColumnName("updated_at");
    }
}
