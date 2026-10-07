using System.Security.Cryptography.X509Certificates;
using Microsoft.Extensions.Configuration;
using SMC.Engine.Core;
using SMC.Engine.Core.Analysis;
using SMC.Engine.Service.Services;

var builder = WebApplication.CreateBuilder(args);

builder.Configuration.AddEnvironmentVariables();

builder.Services.AddGrpc();
builder.Services.AddGrpcReflection();

builder.Services.AddRazorPages();
builder.Services.AddServerSideBlazor();

builder.Services.AddSingleton<DashboardStateService>();
builder.Services.AddSingleton<IngestionTracker>();
builder.Services.AddHostedService<UdpTelemetryListener>();
builder.Services.AddHostedService<UdpTickListener>();
builder.Services.AddHostedService<HistoricalDataLoader>();
builder.Services.AddHttpClient<HistoricalDataLoader>();

var dbHost = Config("EngineDb:Host", "ENGINE_DB_HOST", "localhost");
var dbPort = Config("EngineDb:Port", "ENGINE_DB_PORT", "5432");
var dbName = Config("EngineDb:Database", "ENGINE_DB_NAME", "trading_engine");
var dbUser = Config("EngineDb:Username", "ENGINE_DB_USER", "postgres");
var dbPass = Config("EngineDb:Password", "ENGINE_DB_PASSWORD", "postgres");
var connStr = $"Host={dbHost};Port={dbPort};Database={dbName};Username={dbUser};Password={dbPass};Pooling=true;MinPoolSize=2;MaxPoolSize=50;Connection Idle Lifetime=300;Command Timeout=30;";
Console.WriteLine($"[DB] Built connection string: Host={dbHost};Port={dbPort};Database={dbName}");

builder.Services.AddEngineDbContext(connStr);
builder.Services.AddEngineRepositories();

var caCertPath = Config("GRPC:CaCert", "GRPC_CA_CERT", "deployment/certs/ca.crt");
var engineCertPath = Config("GRPC:EngineCert", "GRPC_ENGINE_CERT", "deployment/certs/server.crt");
var engineKeyPath = Config("GRPC:EngineKey", "GRPC_ENGINE_KEY", "deployment/certs/server.key");
var gatewayTarget = Config("GRPC:GatewayTarget", "GRPC_GATEWAY_TARGET", "https://localhost:50052");

string Config(string configKey, string envKey, string defaultValue)
{
    var env = Environment.GetEnvironmentVariable(envKey);
    if (env is not null) return env;
    var cfg = builder.Configuration[configKey];
    if (cfg is not null) return cfg;
    return defaultValue;
}

using var caCert = X509Certificate2.CreateFromPem(File.ReadAllText(caCertPath));
var engineCert = X509Certificate2.CreateFromPemFile(engineCertPath, engineKeyPath);

builder.WebHost.ConfigureKestrel(options =>
{
    options.Listen(System.Net.IPAddress.Any, 50051, listen =>
    {
        listen.UseHttps(https =>
        {
            https.ServerCertificate = engineCert;
            https.OnAuthenticate = (ctx, sslOptions) =>
            {
                sslOptions.ClientCertificateRequired = true;

                sslOptions.RemoteCertificateValidationCallback = (sender, certificate, chain, errors) =>
                {
                    if (certificate is null)
                        return false;

                    using var customChain = new X509Chain();
                    customChain.ChainPolicy.ExtraStore.Add(caCert);
                    customChain.ChainPolicy.RevocationMode = X509RevocationMode.NoCheck;
                    customChain.ChainPolicy.VerificationFlags = X509VerificationFlags.AllowUnknownCertificateAuthority;
                    return customChain.Build(new X509Certificate2(certificate));
                };
            };
        });
    });

    options.Listen(System.Net.IPAddress.Any, 50053, listen =>
    {
        listen.UseHttps(https =>
        {
            https.ServerCertificate = engineCert;
        });
    });
});

builder.Services.AddExchangeStateClient(gatewayTarget, caCertPath, engineCertPath, engineKeyPath);
builder.Services.AddReconciliation();
builder.Services.AddMarketStructure();
builder.Services.AddSmtDivergence();
builder.Services.AddPoiZone();
builder.Services.AddTriggerLayer();

builder.Services.AddSingleton<EmergencyLoopbackClient>(_ =>
    new EmergencyLoopbackClient(caCertPath, engineCertPath, engineKeyPath));

builder.Services.AddSingleton<InMemorySentimentStore>();
builder.Services.AddSingleton<ISentimentStore>(sp => sp.GetRequiredService<InMemorySentimentStore>());
builder.Services.AddSingleton<IOsintScoreStore>(sp => sp.GetRequiredService<InMemorySentimentStore>());

var app = builder.Build();

app.UseStaticFiles();
app.UseRouting();

app.MapGrpcService<MarketDataService>();
app.MapGrpcService<IntelligenceFeedService>();
app.MapGrpcService<EmergencyServiceImpl>();
app.MapGrpcReflectionService();
app.MapBlazorHub();
app.MapFallbackToPage("/_Host");
app.MapGet("/", () => "SMC Engine gRPC Service");

app.Run();
