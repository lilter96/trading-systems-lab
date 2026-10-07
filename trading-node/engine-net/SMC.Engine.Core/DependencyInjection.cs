using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using SMC.Engine.Core.Analysis;
using SMC.Engine.Core.Data;
using SMC.Engine.Core.GatewayClient;
using SMC.Engine.Core.Repositories;
using SMC.Engine.Core.Services;

namespace SMC.Engine.Core;

public static class DependencyInjection
{
    public static IServiceCollection AddEngineDbContext(this IServiceCollection services, string connectionString)
    {
        services.AddDbContext<EngineDbContext>(options =>
            options.UseNpgsql(connectionString));
        return services;
    }

    public static IServiceCollection AddEngineRepositories(this IServiceCollection services)
    {
        services.AddScoped<ISessionRepository, SessionRepository>();
        services.AddScoped<IOrderRepository, OrderRepository>();
        services.AddScoped<IPositionRepository, PositionRepository>();
        services.AddScoped<IRiskLimitRepository, RiskLimitRepository>();
        services.AddScoped<IStrategyConfigRepository, StrategyConfigRepository>();
        services.AddScoped<IAuditLogRepository, AuditLogRepository>();
        services.AddScoped<IAuditService, AuditService>();
        services.AddScoped<ISystemStateRepository, SystemStateRepository>();
        return services;
    }

    public static IServiceCollection AddExchangeStateClient(this IServiceCollection services, string target, string caCertPath, string clientCertPath, string clientKeyPath)
    {
        services.AddSingleton<IExchangeStateClient>(_ =>
            new ExchangeStateClient(target, caCertPath, clientCertPath, clientKeyPath));
        return services;
    }

    public static IServiceCollection AddReconciliation(this IServiceCollection services)
    {
        services.AddScoped<IReconciliationService, ReconciliationService>();
        services.AddTransient<IHostedService, ReconciliationHostedService>();
        return services;
    }

    public static IServiceCollection AddMarketStructure(this IServiceCollection services)
    {
        services.AddSingleton<IMarketStructureService, MarketStructureService>();
        return services;
    }

    public static IServiceCollection AddSmtDivergence(this IServiceCollection services)
    {
        services.AddSingleton<ISmtDivergenceService, SmtDivergenceService>();
        return services;
    }

    public static IServiceCollection AddPoiZone(this IServiceCollection services)
    {
        services.AddSingleton<IPoiZoneService, PoiZoneService>();
        return services;
    }

    public static IServiceCollection AddTriggerLayer(this IServiceCollection services)
    {
        services.AddSingleton<ITriggerService, TriggerService>();
        return services;
    }
}
