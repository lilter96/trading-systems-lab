using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Design;

namespace SMC.Engine.Core.Data;

public class EngineDbContextFactory : IDesignTimeDbContextFactory<EngineDbContext>
{
    public EngineDbContext CreateDbContext(string[] args)
    {
        var options = new DbContextOptionsBuilder<EngineDbContext>()
            .UseNpgsql("Host=localhost;Database=trading_engine;Username=postgres;Password=postgres")
            .Options;
        return new EngineDbContext(options);
    }
}
