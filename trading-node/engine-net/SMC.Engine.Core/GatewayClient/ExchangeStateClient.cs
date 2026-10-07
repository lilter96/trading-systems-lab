using System.Security.Cryptography.X509Certificates;
using Grpc.Net.Client;
using Trading;

namespace SMC.Engine.Core.GatewayClient;

public interface IExchangeStateClient
{
    Task<ExchangeStateResponse> GetExchangeStateAsync(ExchangeStateRequest request);
}

public class ExchangeStateClient : IExchangeStateClient, IDisposable
{
    private readonly GrpcChannel _channel;
    private readonly ExchangeState.ExchangeStateClient _client;

    public ExchangeStateClient(string target, string caCertPath, string clientCertPath, string clientKeyPath)
    {
        var caCert = X509Certificate2.CreateFromPem(File.ReadAllText(caCertPath));
        var clientCert = X509Certificate2.CreateFromPemFile(clientCertPath, clientKeyPath);

        var handler = new SocketsHttpHandler
        {
            SslOptions = new System.Net.Security.SslClientAuthenticationOptions
            {
                RemoteCertificateValidationCallback = (sender, certificate, chain, errors) =>
                {
                    using var customChain = new X509Chain();
                    customChain.ChainPolicy.ExtraStore.Add(caCert);
                    customChain.ChainPolicy.RevocationMode = X509RevocationMode.NoCheck;
                    customChain.ChainPolicy.VerificationFlags = X509VerificationFlags.AllowUnknownCertificateAuthority;
                    return certificate is not null && customChain.Build(new X509Certificate2(certificate));
                },
                ClientCertificates = new X509CertificateCollection { clientCert },
            },
        };

        _channel = GrpcChannel.ForAddress(target, new GrpcChannelOptions
        {
            HttpHandler = handler,
        });
        _client = new ExchangeState.ExchangeStateClient(_channel);
    }

    public async Task<ExchangeStateResponse> GetExchangeStateAsync(ExchangeStateRequest request)
    {
        return await _client.GetExchangeStateAsync(request);
    }

    public void Dispose()
    {
        _channel?.Dispose();
    }
}
