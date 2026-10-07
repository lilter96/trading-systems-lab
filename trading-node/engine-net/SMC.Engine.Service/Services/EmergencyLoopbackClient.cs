using System.Security.Cryptography.X509Certificates;
using Common;
using Grpc.Net.Client;

namespace SMC.Engine.Service.Services;

public sealed class EmergencyLoopbackClient : IDisposable
{
    private readonly GrpcChannel _channel;
    private readonly EmergencyService.EmergencyServiceClient _client;

    public EmergencyLoopbackClient(string caCertPath, string clientCertPath, string clientKeyPath)
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

        _channel = GrpcChannel.ForAddress("https://localhost:50051", new GrpcChannelOptions
        {
            HttpHandler = handler,
        });
        _client = new EmergencyService.EmergencyServiceClient(_channel);
    }

    public async Task<HaltResponse> HaltSystemAsync(HaltRequest request)
    {
        return await _client.HaltSystemAsync(request);
    }

    public void Dispose()
    {
        _channel?.Dispose();
    }
}
