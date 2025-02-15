package main;

import (
    "fmt";
    "go.starlark.net/starlark";
    "go.starlark.net/resolve";
    "go.starlark.net/starlarkstruct";
    starlarkjson "go.starlark.net/lib/json"
    "log";
    "net";
    "time";
    "math/rand";
    "math";
    "strconv";
    "strings";
    "encoding/json";
    "net/http";
    "io";
    "io/ioutil";
    "os";
    "errors";

    context "context";
    grpc "google.golang.org/grpc";
    keepalive "google.golang.org/grpc/keepalive";

    rpc "archit.us/sandbox";
)

const MaxMessageSize = 1000000;
const TIMEOUT = 3 * time.Second;

type Sandbox struct {
    rpc.SandboxServer;
}

type Author struct {
    Id uint64
    Name string
    AvatarUrl string
    Color string
    Discrim uint32
    Roles []uint64
    Nick string
    Display_name string
    Permissions uint64
}

func make_author(a *rpc.Author) Author {
    new_author := Author{
        Name: a.Name,
        Id: a.Id,
        AvatarUrl: a.AvatarUrl,
        Color: a.Color,
        Discrim: a.Discriminator,
        Roles: a.Roles,
        Nick: a.Nick,
        Permissions: a.Permissions,
    }

    return new_author;
}

/*
 * These next few functions are just used as builtins in the starlark interpreter to add some important features to the language.
 *
 * Not all of the Architus specific builtins can be defined as functions here. Some of them require additional state about the
 * specific auto-response that called them to have full functionality. Therefore, those functions are defined as lambdas
 * further down so that they can capture some state. These functions are just the ones that are "pure"/"stateless" so that a
 * new lambda doesn't have to be declared each time a script needs to be run.
 */
func sin(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
    var rad float64 = 0.0;
    if err := starlark.UnpackArgs(b.Name(), args, kwargs, "rad", &rad); err != nil {
        return nil, err;
    }

    return starlark.Float(math.Sin(rad)), nil;
}

func random(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
    return starlark.Float(rand.Float64()), nil;
}

func randint(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
    var low int = 0;
    var high int = 1;
    if err := starlark.UnpackArgs(b.Name(), args, kwargs, "low", &low, "high", &high); err != nil {
        return nil, err;
    }

    r := rand.Float64();
    var v float64 = float64(high - low);
    v *= r;
    v += float64(low);

    return starlark.MakeInt(int(v)), nil;
}

// Parse user defined headers from a starlark dict into an HTTP request.
func parse_headers(request *http.Request, h starlark.Value) error {
    if h.Type() == "None" {
        return nil;
    }

    if h.Type() != "dict" {
        return errors.New("Second argument must be a dictionary");
    }

    for _, kv := range (h.(*starlark.Dict)).Items() {
        if len(kv) != 2 {
            return errors.New("Dictionary formatted incorrectly. How'd you do that?");
        }
        if kv[0].Type() != "string" || kv[1].Type() != "string" {
            return errors.New("Dictionary must be of type string -> string");
        }
        request.Header.Set(((kv[0]).(starlark.String)).GoString(), ((kv[1]).(starlark.String)).GoString());
    }

    return nil;
}

func (c *Sandbox) RunStarlarkScript(ctx context.Context, in *rpc.StarlarkScript) (*rpc.ScriptOutput, error) {
    const functions = `
p = print
def choice(iterable):
    n = len(iterable)
    if n == 0:
        return None
    i = randint(0, n)
    return iterable[i]
def sum(iterable):
    s = 0
    for i in iterable:
        s += i
    return s
def post(url, headers=None, data=None, j=None):
    if j != None:
        (resp_code, body, is_json) = post_internal(url, headers=headers, json=json.encode(j))
    else:
        (resp_code, body, is_json) = post_internal(url, headers=headers, data=data)
    if is_json:
        json_values = json.decode(body)
        return (resp_code, json_values)
    else:
        return (resp_code, body)
def get(url, headers=None):
    if headers == None:
        (resp_code, body, is_json) = get_internal(url)
    else:
        (resp_code, body, is_json) = get_internal(url, headers)
    if is_json:
        json_values = json.decode(body)
        return (resp_code, json_values)
    else:
        return (resp_code, body)

`;

    // These turn on some extra functionality within the interpreter that allow for some useful things
    // such as while loops, doing things outside of a function, and mutating global values.
    resolve.AllowRecursion = true;
    resolve.AllowNestedDef = true;
    resolve.AllowLambda = true;
    resolve.AllowSet = true;
    resolve.AllowGlobalReassign = true;

    // This is starting to set up the actual script that will be passed to the interpreter.
    var script string = functions;

    // Need to set up a global variable for this before putting it into a struct because it's a list
    // and I don't know how to make a list within a call to sprintf.
    script += "author_roles = [";
    for _, r := range in.MessageAuthor.Roles {
        script += strconv.FormatUint(r, 10) + ", ";
    }
    script += "]\n";

    // Various useful structs that represent aspects of the message that triggered the autoresponse.
    // `struct` is a builtin that is defined later in the program. It comes from the starlark-go repository.
    // For all strings, the variables need to be made in go and then passed into the interpreter so that
    // random special characters don't break everything.
    script += fmt.Sprintf("message = struct(id=%d, content=message_content_full, clean=message_clean_full)\n",
                          in.TriggerMessage.Id);
    script += fmt.Sprintf("author = struct(id=%d, avatar_url=\"%s\", color=\"%s\", discrim=%d, roles=author_roles, name=\"%s\", nick=\"%s\", disp=\"%s\", perms=%d)\n",
                          in.MessageAuthor.Id, in.MessageAuthor.AvatarUrl, in.MessageAuthor.Color, in.MessageAuthor.Discriminator,
                          in.MessageAuthor.Name, in.MessageAuthor.Nick, in.MessageAuthor.DispName, in.MessageAuthor.Permissions);
    script += fmt.Sprintf("channel = struct(id=%d, name=channel_name)\n",
                          in.Channel.Id);
    script += fmt.Sprintf("count = %d\n", in.Count);
    script += "msg = message; a = author; ch = channel;\n";

    var author = make([]starlark.Value, 3);
    author[0] = starlark.String(in.MessageAuthor.Name);
    author[1] = starlark.String(in.MessageAuthor.Nick);
    author[2] = starlark.String(in.MessageAuthor.DispName);

    channel_name := starlark.String(in.Channel.Name);

    var caps = make([]starlark.Value, len(in.Captures));
    for i, c := range in.Captures {
        caps[i] = starlark.String(c);
    }

    var args = make([]starlark.Value, len(in.Arguments));
    for i, c := range in.Arguments {
        args[i] = starlark.String(c);
    }

    // The actual script is no longer put in a main function anymore because with the flags set above in `resolve`
    // we can now get the full functionality of the language outside of a function. This gives the added benefit
    // of allowing users to put newlines in their scripts and not having to do some fancy logic to account for that.
    script += in.Script;

    get_internal := func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
        var raw_url string = "";

        if len(args) < 1 {
            return nil, errors.New("Wrong number of arguments");
        }

        if args[0].Type() != "string" {
            return nil, errors.New("Url must be a string");
        }

        raw_url = ((args[0]).(starlark.String)).GoString();
        req, err := http.NewRequest("GET", raw_url, nil);
        if err != nil {
            return nil, err;
        }

        if len(args) == 2 {
            err = parse_headers(req, args[1]);
            if err != nil {
                return nil, err;
            }
        }

        sauthor := make_author(in.ScriptAuthor);
        mauthor := make_author(in.MessageAuthor);

        mauthor_json, err := json.Marshal(mauthor);
        if err != nil {
            return nil, err;
        }

        sauthor_json, err := json.Marshal(sauthor);
        if err != nil {
            return nil, err;
        }

        req.Header.Set("X-Arch-Author", string(mauthor_json));
        req.Header.Set("X-Arch-Script-Author", string(sauthor_json));
        req.Header.Set("X-Arch-Guild", fmt.Sprintf("%d", in.GuildId))
        req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Architus/1.0; +https://archit.us)");

        var client http.Client;
        resp, err := client.Do(req);
        if err != nil {
            return nil, err;
        }

        limited_body := io.LimitReader(resp.Body, MaxMessageSize);
        bytes, err := io.ReadAll(limited_body);
        if err != nil {
            return nil, err;
        }
        resp.Body.Close();

        json_content := false;
        content, ok := resp.Header["Content-Type"];
        if ok == true {
            for _, v := range content {
                if strings.Contains(v, "application/json") {
                    json_content = true;
                    break;
                }
            }
        }

        resp_code := starlark.MakeInt(resp.StatusCode);
        body := starlark.String(bytes);
        tup := make([]starlark.Value, 3);
        tup[0] = resp_code;
        tup[1] = body;
        tup[2] = starlark.Bool(json_content);
        return starlark.Tuple(tup), nil;
    }

    post_internal := func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
        var raw_url string = "";
        var data string = "";

        if len(args) != 1 {
            return nil, errors.New("You can only pass the url as a positional argument");
        }

        if args[0].Type() != "string" {
            return nil, errors.New("Url must be a string");
        }

        raw_url = ((args[0]).(starlark.String)).GoString();
        req, err := http.NewRequest("POST", raw_url, nil);
        if err != nil {
            return nil, err;
        }

        for _, kw := range kwargs {
            switch ((kw[0]).(starlark.String)).GoString() {
            case "headers":
                err = parse_headers(req, kw[1]);
                if err != nil {
                    return nil, err;
                }
            case "data":
                req.Header.Set("Content-Type", "text/plain");
                data = ((kw[1]).(starlark.String)).GoString();
            case "json":
                req.Header.Set("Content-Type", "application/json");
                data = ((kw[1]).(starlark.String)).GoString()
            }
        }

        mauthor := Author{
            Name: in.MessageAuthor.Name,
            Id: in.MessageAuthor.Id,
            AvatarUrl: in.MessageAuthor.AvatarUrl,
            Color: in.MessageAuthor.Color,
            Discrim: in.MessageAuthor.Discriminator,
            Roles: in.MessageAuthor.Roles,
            Nick: in.MessageAuthor.Nick,
            Permissions: in.MessageAuthor.Permissions,
        }

        sauthor := Author{
            Name: in.ScriptAuthor.Name,
            Id: in.ScriptAuthor.Id,
            AvatarUrl: in.ScriptAuthor.AvatarUrl,
            Color: in.ScriptAuthor.Color,
            Discrim: in.ScriptAuthor.Discriminator,
            Roles: in.ScriptAuthor.Roles,
            Nick: in.ScriptAuthor.Nick,
            Permissions: in.ScriptAuthor.Permissions,
        }

        mauthor_json, err := json.Marshal(mauthor);
        if err != nil {
            return nil, err;
        }

        sauthor_json, err := json.Marshal(sauthor);
        if err != nil {
            return nil, err;
        }
        req.Header.Set("X-Arch-Author", string(mauthor_json));
        req.Header.Set("X-Arch-Script-Author", string(sauthor_json));
        req.Header.Set("X-Arch-Guild", fmt.Sprintf("%d", in.GuildId))
        req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Architus/1.0; +https://archit.us)");

        req.Body = ioutil.NopCloser(strings.NewReader(data));

        var client http.Client;
        resp, err := client.Do(req);
        if err != nil {
            return nil, err;
        }

        limited_body := io.LimitReader(resp.Body, MaxMessageSize);
        bytes, err := io.ReadAll(limited_body);
        if err != nil {
            return nil, err;
        }
        resp.Body.Close();

        json_content := false;
        content, ok := resp.Header["Content-Type"];
        if ok == true {
            for _, v := range content {
                if strings.Contains(v, "application/json") {
                    json_content = true;
                    break;
                }
            }
        }

        resp_code := starlark.MakeInt(resp.StatusCode);
        body := starlark.String(bytes);
        tup := make([]starlark.Value, 3);
        tup[0] = resp_code;
        tup[1] = starlark.String(body);
        tup[2] = starlark.Bool(json_content);
        return starlark.Tuple(tup), nil;
    }

    // This tells the interpreter what all of our builtins are. Struct is a starlark-go specific functionality that
    // allows for creating a struct from kwarg values.
    // TODO(jjohnson): Add get and post builtins
    predeclared := starlark.StringDict{
        "random": starlark.NewBuiltin("random", random),
        "randint": starlark.NewBuiltin("randint", randint),
        "sin": starlark.NewBuiltin("sin", sin),
        "struct": starlark.NewBuiltin("struct", starlarkstruct.Make),
        "post_internal": starlark.NewBuiltin("post_internal", post_internal),
        "get_internal": starlark.NewBuiltin("get_internal", get_internal),
        "message_content_full": starlark.String(in.TriggerMessage.Content),
        "message_clean_full": starlark.String(in.TriggerMessage.Clean),
        "caps": starlark.NewList(caps),
        "args": starlark.NewList(args),
        "auth_list": starlark.NewList(author),
        "channel_name": channel_name,
        "json": starlarkjson.Module,
    };

    var messages []string;
    thread := &starlark.Thread{
        Name: "sandbox_thread",
        Print: func(_ *starlark.Thread, msg string) { messages = append(messages, msg); },
    };

    starChan := make(chan error, 1);
    // _, runtime_err := starlark.ExecFile(thread, script_name, nil, nil);
    go func() {
        _, tmpE := starlark.ExecFile(thread, "sandbox_script.star", script, predeclared);
        starChan <- tmpE;
    }();

    var runtime_err error;
    select {
    case runtime_err = <- starChan:
        log.Print(runtime_err);
    case <- time.After(TIMEOUT):
        log.Print("Script timed out");
        return &rpc.ScriptOutput{
            Output: "",
            Error: "Script timed out",
            Errno: 5,
        }, nil;
    }

    if runtime_err != nil {
        log.Print("Script failed to run");
        log.Print(runtime_err.Error());
        return &rpc.ScriptOutput{
            Output: "",
            Error: runtime_err.Error(),
            Errno: 4,
        }, nil;
    }

    return &rpc.ScriptOutput{
        Output: strings.Join(messages, "\n"),
        Error: "",
        Errno: 0,
    }, nil;
}

func newServer() *Sandbox {
    return &Sandbox{};
}

func main() {
    lis, sock_err := net.Listen("tcp", "0.0.0.0:1337");
    if sock_err != nil {
        log.Fatal("Failed to connect to socket");
    }

    grpcServer := grpc.NewServer(
        grpc.KeepaliveParams(
            keepalive.ServerParameters{
                Time:       (time.Duration(20) * time.Second),
                Timeout:    (time.Duration(5)  * time.Second),
            },
        ),
        grpc.KeepaliveEnforcementPolicy(
            keepalive.EnforcementPolicy{
                MinTime:                (time.Duration(15) * time.Second),
                PermitWithoutStream:    true,
            },
        ),
    );
    rpc.RegisterSandboxServer(grpcServer, newServer());
    proxy := os.Getenv("HTTP_PROXY");
    if proxy != "" {
        fmt.Print("Starting production server with proxy: ");
        fmt.Println(proxy);
    } else {
        fmt.Println("Starting debug server without a proxy");
    }
    grpcServer.Serve(lis);
}
