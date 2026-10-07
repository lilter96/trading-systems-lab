namespace SMC.Engine.Core.Repositories;

public interface ISystemStateRepository
{
    Task SetHaltedAsync(CancellationToken ct = default);
}
