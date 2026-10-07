using Microsoft.EntityFrameworkCore;
using SMC.Engine.Core.Data;
using SMC.Engine.Core.Entities;

namespace SMC.Engine.Core.Repositories;

public class SystemStateRepository : ISystemStateRepository
{
    private readonly EngineDbContext _db;

    public SystemStateRepository(EngineDbContext db)
    {
        _db = db;
    }

    public async Task SetHaltedAsync(CancellationToken ct = default)
    {
        var state = await _db.SystemStates
            .FirstOrDefaultAsync(x => x.SystemId == "primary_instance", ct);

        if (state is null)
        {
            state = new SystemState
            {
                SystemId = "primary_instance",
                State = "HALTED",
                TradingEnabled = false,
                UpdatedAt = DateTimeOffset.UtcNow,
            };
            _db.SystemStates.Add(state);
        }
        else
        {
            state.State = "HALTED";
            state.TradingEnabled = false;
            state.UpdatedAt = DateTimeOffset.UtcNow;
        }

        await _db.SaveChangesAsync(ct);
    }
}
