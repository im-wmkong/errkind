import argparse
import json
import os
from pathlib import Path
import subprocess
import tempfile
import zipfile


CONSUMER = r'''package main

import (
    "bytes"
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "log/slog"
    "net/http/httptest"
    "reflect"
    "strings"

    "github.com/im-wmkong/errkind"
    grpcint "github.com/im-wmkong/errkind/grpc"
    httpint "github.com/im-wmkong/errkind/http"
    logrusint "github.com/im-wmkong/errkind/logrus"
    otelint "github.com/im-wmkong/errkind/otel"
    slogint "github.com/im-wmkong/errkind/slog"
    zapint "github.com/im-wmkong/errkind/zap"
    zerologint "github.com/im-wmkong/errkind/zerolog"
    "github.com/rs/zerolog"
    "github.com/sirupsen/logrus"
    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
    "google.golang.org/grpc/codes"
)

func main() {
    r := errkind.NewRegistry()
    k := r.Define(101, "remote")
    local := k.New("secret", errkind.With("token", "secret"))
    st := grpcint.ToStatus(local, grpcint.Identity(), grpcint.Message("public"), grpcint.Field("uid", "42"))
    remote := grpcint.FromStatus(st, r)
    if c, ok := errkind.CodeOf(remote); !ok || c != 101 || !errors.Is(remote, k) { panic("identity or code") }
    if errors.Is(grpcint.FromStatus(st, nil), k) { panic("implicit registry binding") }
    rec := httptest.NewRecorder()
    if err := httpint.Write(rec, local); err != nil || rec.Code != 500 || rec.Body.String() != "{\"message\":\"Internal Server Error\"}\n" { panic("HTTP disclosure") }
    if st := grpcint.ToStatus(local); st.Message() != "Internal Server Error" || len(st.Details()) != 0 { panic("gRPC disclosure") }
    found := false
    for _, attr := range otelint.Attributes(remote) {
        if string(attr.Key) == "err.code" && attr.Value.AsInt64() == 101 { found = true }
    }
    if !found { panic("otel code") }
    tree := errors.Join(remote, k.Wrap(errors.New("database failure"), "lookup user", errkind.With("uid", "other"), errkind.With("bad", make(chan int))))
    for _, err := range []error{nil, errors.New("plain"), remote, tree} {
        var zapBuf, zeroBuf, slogBuf, logrusBuf bytes.Buffer
        zapLogger := zap.New(zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), zapcore.AddSync(&zapBuf), zap.DebugLevel))
        zapLogger.Error("failed", zapint.Err(err))
        zeroLogger := zerolog.New(&zeroBuf)
        zeroLogger.Error().Func(zerologint.Err(err)).Msg("failed")
        slog.New(slog.NewJSONHandler(&slogBuf, nil)).Error("failed", slogint.Err(err))
        logrusLogger := logrus.New()
        logrusLogger.Out = &logrusBuf
        logrusLogger.Formatter = &logrus.JSONFormatter{}
        logrusLogger.WithFields(logrusint.Fields(err)).Error("failed")
        var baseline any
        for i, output := range []string{slogBuf.String(), zapBuf.String(), zeroBuf.String(), logrusBuf.String()} {
            var fields map[string]any
            if e := json.Unmarshal([]byte(output), &fields); e != nil { panic(e) }
            if i == 0 { baseline = fields["err"] } else if !reflect.DeepEqual(baseline, fields["err"]) { panic("logger schemas differ") }
            if err == tree && (!strings.Contains(output, `"uid":"42"`) || !strings.Contains(output, `"uid":"other"`) || !strings.Contains(output, "database failure")) { panic("tree diagnostics lost") }
        }
        if err == tree {
            root := baseline.(map[string]any)
            if root["code"] != nil || len(root["causes"].([]any)) != 2 { panic("ambiguous primary") }
        }
    }
    if grpcint.ToStatus(context.DeadlineExceeded).Code() != codes.DeadlineExceeded { panic("deadline") }
    fmt.Println("Four logger schemas match; HTTP, gRPC, OTel and registry binding validated.")
}
'''


def verify(version):
    root = Path(__file__).resolve().parents[1]
    goroot = subprocess.check_output(['go', 'env', 'GOROOT'], text=True).strip()
    go = str(Path(goroot) / 'bin' / 'go')
    modules = ['', 'grpc', 'otel', 'zap', 'zerolog', 'logrus']
    with tempfile.TemporaryDirectory(prefix='errkind-release-') as directory:
        base = Path(directory)
        proxy = base / 'proxy'
        for relative in modules:
            source = root / relative
            module = 'github.com/im-wmkong/errkind' + ('/' + relative if relative else '')
            destination = proxy / module / '@v'
            destination.mkdir(parents=True)
            (destination / (version + '.mod')).write_bytes((source / 'go.mod').read_bytes())
            (destination / (version + '.info')).write_text(json.dumps({'Version': version, 'Time': '2026-09-11T00:00:00Z'}))
            (destination / 'list').write_text(version + '\n')
            with zipfile.ZipFile(destination / (version + '.zip'), 'w', zipfile.ZIP_DEFLATED) as archive:
                for directory, dirs, files in os.walk(source):
                    current = Path(directory)
                    dirs[:] = [d for d in dirs if not d.startswith('.') and d != 'vendor' and not (current / d / 'go.mod').exists()]
                    for name in files:
                        if name.endswith(('.go', '.md', '.sh', '.py')) or name in ('go.mod', 'go.sum', 'LICENSE'):
                            path = current / name
                            archive.write(path, module + '@' + version + '/' + path.relative_to(source).as_posix())
        consumer = base / 'consumer'
        consumer.mkdir()
        requires = '\n'.join('\tgithub.com/im-wmkong/errkind' + ('/' + relative if relative else '') + ' ' + version for relative in modules)
        (consumer / 'go.mod').write_text('module candidate-consumer\n\ngo 1.25.0\n\nrequire (\n' + requires + '\n)\n')
        (consumer / 'main.go').write_text(CONSUMER)
        env = os.environ.copy()
        env.update({
            'GOWORK': 'off',
            'GOTOOLCHAIN': 'local',
            'GOROOT': goroot,
            'GOPROXY': proxy.as_uri() + ',https://proxy.golang.org',
            'GONOSUMDB': 'github.com/im-wmkong/errkind*',
            'GONOPROXY': 'none',
            'GOMODCACHE': str(base / 'modcache'),
            'GOCACHE': str(base / 'buildcache'),
            'GOBIN': str(base / 'bin'),
        })
        for args in [['mod', 'tidy'], ['build', './...'], ['run', '.'],
                     ['install', 'github.com/im-wmkong/errkind/cmd/errkind@' + version],
                     ['install', 'github.com/im-wmkong/errkind/cmd/errkindlint@' + version]]:
            subprocess.run([go, *args], cwd=consumer, env=env, check=True)
        result = subprocess.check_output([go, 'list', '-m', '-json', 'all'], cwd=consumer, env=env, text=True)
        if '"Replace"' in result:
            raise RuntimeError('consumer unexpectedly used replace')
        print('Candidate validation passed with zero replaces and an isolated module cache; nothing was published.')


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--version', default='v0.2.0')
    verify(parser.parse_args().version)
