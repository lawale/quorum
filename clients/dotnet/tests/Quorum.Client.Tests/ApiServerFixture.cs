using System.Diagnostics;

namespace Quorum.Client.Tests;

/// <summary>
/// Builds and runs the Quorum test server (the Go apitest harness as a process) for the whole
/// collection. At teardown the server exits non-zero if any traffic disagreed with the OpenAPI
/// document or any documented operation was never called, which fails the run.
/// </summary>
public sealed class ApiServerFixture : IDisposable
{
    private readonly Process _server;
    private readonly string _buildDir;
    private readonly System.Text.StringBuilder _stderr = new();

    public Uri Url { get; }

    public ApiServerFixture()
    {
        var repoRoot = FindRepoRoot();
        _buildDir = Directory.CreateTempSubdirectory("quorum-apiserver-").FullName;
        var bin = Path.Combine(_buildDir, "apiserver");
        Run("go", $"build -o \"{bin}\" ./internal/apitest/cmd/apiserver", repoRoot);

        _server = new Process
        {
            StartInfo = new ProcessStartInfo(bin)
            {
                WorkingDirectory = repoRoot,
                RedirectStandardInput = true,
                RedirectStandardOutput = true,
                RedirectStandardError = true,
                UseShellExecute = false,
            },
        };
        _server.ErrorDataReceived += (_, e) =>
        {
            if (e.Data is null) return;
            lock (_stderr) _stderr.AppendLine(e.Data);
            Console.Error.WriteLine(e.Data);
        };
        _server.Start();
        _server.BeginErrorReadLine();

        var line = _server.StandardOutput.ReadLine();
        if (string.IsNullOrWhiteSpace(line))
        {
            _server.WaitForExit(5000);
            throw new InvalidOperationException($"apiserver exited before reporting its URL\n{_stderr}");
        }
        Url = new Uri(line.Trim());
    }

    public void Dispose()
    {
        // Closing stdin asks the server to stop and print its report.
        _server.StandardInput.Close();
        if (!_server.WaitForExit(30_000))
        {
            _server.Kill();
            throw new InvalidOperationException("apiserver did not stop");
        }
        var code = _server.ExitCode;
        _server.Dispose();
        try { Directory.Delete(_buildDir, recursive: true); } catch { }
        if (code != 0)
            throw new InvalidOperationException($"apiserver reported contract failures (exit {code}):\n{_stderr}");
    }

    private static void Run(string file, string args, string cwd)
    {
        var p = Process.Start(new ProcessStartInfo(file, args) { WorkingDirectory = cwd, UseShellExecute = false })
            ?? throw new InvalidOperationException($"could not start {file}");
        p.WaitForExit();
        if (p.ExitCode != 0) throw new InvalidOperationException($"{file} {args} failed with exit {p.ExitCode}");
    }

    private static string FindRepoRoot()
    {
        var dir = new DirectoryInfo(AppContext.BaseDirectory);
        while (dir is not null && !File.Exists(Path.Combine(dir.FullName, "go.mod"))) dir = dir.Parent;
        return dir?.FullName ?? throw new InvalidOperationException("go.mod not found above the test binary");
    }
}

[CollectionDefinition(nameof(ApiServerCollection))]
public sealed class ApiServerCollection : ICollectionFixture<ApiServerFixture>;
