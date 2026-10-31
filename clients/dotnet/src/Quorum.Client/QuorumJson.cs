using System.Text.Json;
using System.Text.Json.Serialization;

namespace Quorum.Client;

/// <summary>Serializer settings for the API's own types.</summary>
public static class QuorumJson
{
    /// <summary>Settings the client uses for API bodies: snake_case names and enums, nulls omitted.</summary>
    public static JsonSerializerOptions Options { get; } = Create(strict: false);

    /// <summary>Settings for payloads and metadata when none are given: ASP.NET Core's web defaults (camelCase).</summary>
    public static JsonSerializerOptions DefaultPayloadOptions { get; } = new(JsonSerializerDefaults.Web);

    internal static JsonSerializerOptions Create(bool strict)
    {
        var o = new JsonSerializerOptions
        {
            PropertyNamingPolicy = JsonNamingPolicy.SnakeCaseLower,
            DefaultIgnoreCondition = JsonIgnoreCondition.WhenWritingNull,
            PropertyNameCaseInsensitive = false,
        };
        o.Converters.Add(new JsonStringEnumConverter(JsonNamingPolicy.SnakeCaseLower));
        if (strict)
        {
            o.UnmappedMemberHandling = JsonUnmappedMemberHandling.Disallow;
        }
        return o;
    }
}

/// <summary>Helpers for the Go duration strings the API uses, such as <c>24h</c> or <c>1h30m</c>.</summary>
public static class Duration
{
    /// <summary>Formats a <see cref="TimeSpan"/> as a Go duration string, for <c>auto_expire_duration</c>.</summary>
    public static string Format(TimeSpan value)
    {
        if (value == TimeSpan.Zero) return "0s";
        var sb = new System.Text.StringBuilder();
        if (value < TimeSpan.Zero) { sb.Append('-'); value = value.Negate(); }
        var hours = (long)Math.Floor(value.TotalHours);
        if (hours > 0) sb.Append(hours).Append('h');
        if (value.Minutes > 0) sb.Append(value.Minutes).Append('m');
        var seconds = value.Seconds + value.Milliseconds / 1000.0;
        if (seconds > 0 || sb.Length == 0 || sb.ToString() == "-")
            sb.Append(seconds.ToString(System.Globalization.CultureInfo.InvariantCulture)).Append('s');
        return sb.ToString();
    }

    /// <summary>Parses a Go duration string (<c>h</c>, <c>m</c>, <c>s</c>, <c>ms</c> units; a <c>d</c> unit is accepted as 24h).</summary>
    public static TimeSpan Parse(string value)
    {
        if (!TryParse(value, out var result)) throw new FormatException($"'{value}' is not a Go duration");
        return result;
    }

    /// <summary>Tries to parse a Go duration string.</summary>
    public static bool TryParse(string? value, out TimeSpan result)
    {
        result = TimeSpan.Zero;
        if (string.IsNullOrEmpty(value)) return false;
        var s = value.AsSpan();
        var negative = false;
        if (s[0] == '-') { negative = true; s = s[1..]; }
        else if (s[0] == '+') s = s[1..];
        if (s.Length == 0) return false;
        if (s.SequenceEqual("0")) return true;

        double totalSeconds = 0;
        while (s.Length > 0)
        {
            var i = 0;
            while (i < s.Length && (char.IsAsciiDigit(s[i]) || s[i] == '.')) i++;
            if (i == 0) return false;
            if (!double.TryParse(s[..i], System.Globalization.NumberStyles.Float, System.Globalization.CultureInfo.InvariantCulture, out var n)) return false;
            s = s[i..];
            var j = 0;
            while (j < s.Length && !char.IsAsciiDigit(s[j]) && s[j] != '.') j++;
            if (j == 0) return false;
            var unit = s[..j];
            s = s[j..];
            double mult;
            if (unit.SequenceEqual("ns")) mult = 1e-9;
            else if (unit.SequenceEqual("us") || unit.SequenceEqual("µs")) mult = 1e-6;
            else if (unit.SequenceEqual("ms")) mult = 1e-3;
            else if (unit.SequenceEqual("s")) mult = 1;
            else if (unit.SequenceEqual("m")) mult = 60;
            else if (unit.SequenceEqual("h")) mult = 3600;
            else if (unit.SequenceEqual("d")) mult = 86400;
            else return false;
            totalSeconds += n * mult;
        }
        result = TimeSpan.FromSeconds(negative ? -totalSeconds : totalSeconds);
        return true;
    }
}
