using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata.Builders;
using SMC.Engine.Core.Entities;

namespace SMC.Engine.Core.Data.EntityConfigurations;

public class AuditLogEntryConfiguration : IEntityTypeConfiguration<AuditLogEntry>
{
    public void Configure(EntityTypeBuilder<AuditLogEntry> builder)
    {
        builder.ToTable("audit_log");
        builder.HasKey(x => x.Id);
        builder.Property(x => x.Id).HasColumnName("id").ValueGeneratedOnAdd();
        builder.Property(x => x.SessionId).HasColumnName("session_id");
        builder.Property(x => x.EventType).HasColumnName("event_type").HasMaxLength(50).IsRequired();
        builder.Property(x => x.CorrelationId).HasColumnName("correlation_id");
        builder.Property(x => x.Payload).HasColumnName("payload").HasColumnType("jsonb").IsRequired();
        builder.Property(x => x.CreatedAt).HasColumnName("created_at");

        builder.HasOne(x => x.Session)
            .WithMany(s => s.AuditLogs)
            .HasForeignKey(x => x.SessionId)
            .OnDelete(DeleteBehavior.Restrict);
    }
}
