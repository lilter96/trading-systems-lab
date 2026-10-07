using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata.Builders;
using SMC.Engine.Core.Entities;

namespace SMC.Engine.Core.Data.EntityConfigurations;

public class OrderConfiguration : IEntityTypeConfiguration<Order>
{
    public void Configure(EntityTypeBuilder<Order> builder)
    {
        builder.ToTable("orders");
        builder.HasKey(x => x.Id);
        builder.Property(x => x.Id).HasColumnName("id");
        builder.Property(x => x.SessionId).HasColumnName("session_id");
        builder.Property(x => x.ExchangeOrderId).HasColumnName("exchange_order_id").HasMaxLength(100);
        builder.Property(x => x.Asset).HasColumnName("asset").HasMaxLength(20).IsRequired();
        builder.Property(x => x.Direction).HasColumnName("direction").HasMaxLength(10).IsRequired();
        builder.Property(x => x.Size).HasColumnName("size").HasColumnType("decimal(18,8)");
        builder.Property(x => x.Status).HasColumnName("status").HasMaxLength(20).IsRequired();
        builder.Property(x => x.ExecutionPrice).HasColumnName("execution_price").HasColumnType("decimal(18,8)");
        builder.Property(x => x.OrderType).HasColumnName("order_type").HasMaxLength(20).IsRequired();
        builder.Property(x => x.LimitPrice).HasColumnName("limit_price").HasColumnType("decimal(18,8)");
        builder.Property(x => x.StopPrice).HasColumnName("stop_price").HasColumnType("decimal(18,8)");
        builder.Property(x => x.CreatedAt).HasColumnName("created_at");
        builder.Property(x => x.UpdatedAt).HasColumnName("updated_at");
        builder.Property(x => x.FilledAt).HasColumnName("filled_at");
        builder.Property(x => x.RejectedReason).HasColumnName("rejected_reason");

        builder.HasOne(x => x.Session)
            .WithMany(s => s.Orders)
            .HasForeignKey(x => x.SessionId)
            .OnDelete(DeleteBehavior.Restrict);
    }
}
