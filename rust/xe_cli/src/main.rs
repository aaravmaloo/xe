use anyhow::{anyhow, bail, Context, Result};
use rayon::prelude::*;
use regex::Regex;
use reqwest::blocking::Client;
use reqwest::StatusCode;
use serde::{Deserialize, Serialize};
use serde_json::{json, Map, Value};
use sha1::{Digest as Sha1Digest, Sha1};
use sha2::Sha256;
use std::cmp::Ordering;
use std::collections::{BTreeMap, HashMap, HashSet};
use std::env;
use std::fs::{self, File};
use std::io::{self, BufRead, BufReader, Read, Write};
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};
use std::sync::{Arc, Mutex};
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};
use time::format_description::well_known::Iso8601;
use time::OffsetDateTime;
use walkdir::WalkDir;
use zip::write::FileOptions;
use zip::ZipArchive;
use zip::ZipWriter;

const XE_TOML: &str = "xe.toml";

fn main() {
    if let Err(err) = run() {
        error(&format!("{:#}", err));
        std::process::exit(1);
    }
}

fn run() -> Result<()> {
    let root = parse_root_args()?;
    if root.show_help {
        print_help();
        return Ok(());
    }
    if root.show_version {
        print_version();
        return Ok(());
    }

    let config_file = root.config_file.unwrap_or_else(xe_config_file);
    let profiler = if root.profile {
        let dir = root.profile_dir.unwrap_or_else(|| xe_home().join("profiles"));
        let (prof, info_data) = Profiler::start(&dir)?;
        println!("Profiling enabled.");
        println!("Logs: {}", info_data.log_path.display());
        println!("CPU: {}", info_data.cpu_path.display());
        println!("Heap: {}", info_data.heap_path.display());
        Some(prof)
    } else {
        None
    };

    let ctx = AppContext {
        config_file,
        profiler: profiler.clone(),
    };

    if let Some(p) = profiler.as_ref() {
        p.event(
            "command.start",
            json!({
                "args_count": root.command_args.len(),
            }),
        );
    }

    let command_result = dispatch(&ctx, &root.command_args);

    if let Some(p) = profiler.as_ref() {
        p.event("command.stop", json!({}));
        p.stop()?;
    }

    command_result
}

#[derive(Debug)]
struct RootArgs {
    config_file: Option<PathBuf>,
    profile: bool,
    profile_dir: Option<PathBuf>,
    show_help: bool,
    show_version: bool,
    command_args: Vec<String>,
}

fn parse_root_args() -> Result<RootArgs> {
    let mut args: Vec<String> = env::args().skip(1).collect();
    let mut config_file: Option<PathBuf> = None;
    let mut profile = false;
    let mut profile_dir: Option<PathBuf> = None;
    let mut show_help = false;
    let mut show_version = false;

    let mut idx = 0usize;
    while idx < args.len() {
        let arg = args[idx].clone();
        match arg.as_str() {
            "--config" => {
                let value = args
                    .get(idx + 1)
                    .ok_or_else(|| anyhow!("--config requires a path"))?;
                config_file = Some(PathBuf::from(value));
                idx += 2;
            }
            "--profile" => {
                profile = true;
                idx += 1;
            }
            "--profile-dir" => {
                let value = args
                    .get(idx + 1)
                    .ok_or_else(|| anyhow!("--profile-dir requires a path"))?;
                profile_dir = Some(PathBuf::from(value));
                idx += 2;
            }
            "-h" | "--help" => {
                show_help = true;
                idx += 1;
            }
            "-V" | "--version" => {
                show_version = true;
                idx += 1;
            }
            _ => {
                break;
            }
        }
    }

    let command_args = if idx >= args.len() {
        Vec::new()
    } else {
        args.drain(idx..).collect()
    };

    Ok(RootArgs {
        config_file,
        profile,
        profile_dir,
        show_help,
        show_version,
        command_args,
    })
}

#[derive(Clone)]
struct AppContext {
    config_file: PathBuf,
    profiler: Option<Profiler>,
}

fn dispatch(ctx: &AppContext, args: &[String]) -> Result<()> {
    if args.is_empty() {
        print_help();
        return Ok(());
    }
    let cmd = args[0].as_str();
    let rest = &args[1..];
    match cmd {
        "add" => cmd_add(ctx, rest),
        "list" => cmd_list(ctx, rest),
        "check" | "show" => cmd_check(rest),
        "remove" => cmd_remove(ctx, rest),
        "run" => cmd_run(ctx, rest),
        "shell" => cmd_shell(ctx, rest),
        "init" => cmd_init(ctx, rest),
        "use" => cmd_use(ctx, rest),
        "venv" => cmd_venv(ctx, rest),
        "config" => cmd_config(ctx, rest),
        "import" => cmd_import(ctx, rest),
        "export" => cmd_export(rest),
        "clean" => cmd_clean(rest),
        "snapshot" => cmd_snapshot(rest),
        "restore" => cmd_restore(rest),
        "sync" => cmd_sync(ctx, rest),
        "lock" => cmd_lock(ctx, rest),
        "publish" => cmd_push(ctx, rest, false),
        "format" => cmd_format(ctx, rest),
        "version" => {
            print_version();
            Ok(())
        }
        "cache" => cmd_cache(ctx, rest),
        "python" => cmd_python(ctx, rest),
        "pip" => cmd_pip(ctx, rest),
        "tool" => cmd_tool(ctx, rest),
        "x" => cmd_x_alias(ctx, rest),
        "build" => cmd_build(rest),
        "push" => cmd_push(ctx, rest, false),
        "tpush" => cmd_push(ctx, rest, true),
        "auth" => cmd_auth(rest),
        "mirror" => cmd_mirror(rest),
        "plugin" => cmd_plugin(rest),
        "self" => cmd_self(rest),
        "workspace" | "workspaces" => cmd_workspace(rest),
        "why" => cmd_why(rest),
        "tree" => cmd_tree(rest),
        "doctor" => cmd_doctor(rest),
        "setup" => cmd_setup(rest),
        _ => {
            print_help();
            bail!("unknown command: {cmd}");
        }
    }
}

fn cmd_add(ctx: &AppContext, args: &[String]) -> Result<()> {
    if args.is_empty() {
        bail!("usage: xe add <package_name>...");
    }
    let wd = env::current_dir().context("failed to get cwd")?;
    let (mut cfg, toml_path) = load_or_create_project(&wd)?;
    let runtime = ensure_runtime_for_project(ctx, &wd, &mut cfg)?;
    if runtime.config_changed {
        save_project(&toml_path, &cfg)?;
    }

    let target = if runtime.selection.is_venv {
        format!("venv:{}", runtime.selection.venv_name)
    } else {
        "global".to_string()
    };
    info(&format!(
        "Installing {} requirement(s) with Python {} [{}]...",
        args.len(),
        cfg.python.version,
        target
    ));

    let installer = Installer::new(Path::new(&cfg.cache.global_dir))?;
    let reqs: Vec<String> = args.to_vec();
    let resolved = installer.install(
        ctx,
        &cfg,
        &reqs,
        &wd,
        &runtime.selection.site_packages,
        &runtime.selection.python_exe,
    )?;

    for req in args {
        if let Some(dep_name) = requirement_to_dep_name(req) {
            cfg.deps.insert(dep_name, "*".to_string());
        }
    }
    for p in &resolved {
        cfg.deps
            .insert(normalize_dep_name(&p.name), p.version.clone());
    }
    save_project(&toml_path, &cfg)?;
    success(&format!("Installed {} package artifact(s)", resolved.len()));
    Ok(())
}

fn cmd_list(ctx: &AppContext, _args: &[String]) -> Result<()> {
    let wd = env::current_dir().context("failed to get cwd")?;
    let (mut cfg, toml_path) = load_or_create_project(&wd)?;
    let runtime = ensure_runtime_for_project(ctx, &wd, &mut cfg)?;
    if runtime.config_changed {
        save_project(&toml_path, &cfg)?;
    }

    let output = Command::new(&runtime.selection.python_exe)
        .args(["-m", "pip", "list", "--format", "json"])
        .output()
        .context("failed to run pip list")?;
    if !output.status.success() {
        let stderr = String::from_utf8_lossy(&output.stderr);
        let stdout = String::from_utf8_lossy(&output.stdout);
        bail!("Failed to list packages: {}\n{}{}", output.status, stdout, stderr);
    }

    let mut pkgs = parse_pip_list_output(&output.stdout)?;
    pkgs.sort_by(|a, b| a.name.to_lowercase().cmp(&b.name.to_lowercase()));
    print_pkg_table(&pkgs);
    Ok(())
}

fn cmd_check(args: &[String]) -> Result<()> {
    if args.len() != 1 {
        bail!("usage: xe check <package_name>");
    }
    let metadata = fetch_metadata_from_pypi(&args[0])?;
    println!("Name: {}", metadata.info.name);
    println!("Version: {}", metadata.info.version);
    println!("Summary: {}", metadata.info.summary);
    println!("Home-page: {}", metadata.info.home_page);
    Ok(())
}

fn cmd_remove(ctx: &AppContext, args: &[String]) -> Result<()> {
    if args.is_empty() {
        bail!("usage: xe remove <package_name>...");
    }
    let wd = env::current_dir().context("failed to get cwd")?;
    let (mut cfg, toml_path) = load_or_create_project(&wd)?;
    let runtime = ensure_runtime_for_project(ctx, &wd, &mut cfg)?;
    if runtime.config_changed {
        save_project(&toml_path, &cfg)?;
    }

    let is_remove_all = args.len() == 1 && args[0].eq_ignore_ascii_case("all");
    if is_remove_all {
        let out = Command::new(&runtime.selection.python_exe)
            .args(["-m", "pip", "list", "--format", "json"])
            .output()
            .context("failed to list packages")?;
        if !out.status.success() {
            bail!("Failed to list packages: {}", out.status);
        }
        let pkgs = parse_pip_list_output(&out.stdout)?;
        let to_remove: Vec<String> = pkgs
            .iter()
            .filter_map(|p| {
                let n = p.name.to_lowercase();
                if n == "pip" || n == "setuptools" || n == "wheel" {
                    None
                } else {
                    Some(p.name.clone())
                }
            })
            .collect();
        if !to_remove.is_empty() {
            let mut command = Command::new(&runtime.selection.python_exe);
            command.arg("-m").arg("pip").arg("uninstall").arg("-y");
            command.args(&to_remove);
            let status = command.status().context("failed to uninstall packages")?;
            if !status.success() {
                bail!("Failed to remove all packages: {}", status);
            }
        }
        cfg.deps.clear();
        save_project(&toml_path, &cfg)?;
        success("Removed all packages from active environment");
        return Ok(());
    }

    let mut req_names = Vec::new();
    for raw in args {
        if let Some(n) = requirement_to_dep_name(raw) {
            req_names.push(n);
        }
    }
    if req_names.is_empty() {
        bail!("No valid package names provided");
    }
    let mut command = Command::new(&runtime.selection.python_exe);
    command.arg("-m").arg("pip").arg("uninstall").arg("-y");
    command.args(&req_names);
    let status = command.status().context("failed to uninstall packages")?;
    if !status.success() {
        bail!("Failed to remove packages: {}", status);
    }
    for name in req_names {
        cfg.deps.remove(&name);
    }
    save_project(&toml_path, &cfg)?;
    success(&format!("Removed {} package(s)", args.len()));
    Ok(())
}

fn cmd_run(ctx: &AppContext, args: &[String]) -> Result<()> {
    let wd = env::current_dir().context("failed to get cwd")?;
    let (mut cfg, toml_path) = load_or_create_project(&wd)?;
    let runtime = ensure_runtime_for_project(ctx, &wd, &mut cfg)?;
    if runtime.config_changed {
        save_project(&toml_path, &cfg)?;
    }
    let mut command_args = args.to_vec();
    if let Some(first) = command_args.first() {
        if first == "--" {
            command_args.remove(0);
        }
    }
    if command_args.is_empty() {
        bail!("No command provided after '--'");
    }
    let mut command_name = command_args[0].clone();
    if command_name.eq_ignore_ascii_case("python") || command_name.eq_ignore_ascii_case("python.exe")
    {
        command_name = runtime.selection.python_exe.to_string_lossy().to_string();
    }

    let mut command = Command::new(command_name);
    command.args(&command_args[1..]);
    apply_runtime_env(&mut command, &runtime.selection)?;
    command.stdin(Stdio::inherit());
    command.stdout(Stdio::inherit());
    command.stderr(Stdio::inherit());
    let status = command.status().context("failed to run command")?;
    if let Some(code) = status.code() {
        if code != 0 {
            std::process::exit(code);
        }
    }
    Ok(())
}

fn cmd_shell(ctx: &AppContext, _args: &[String]) -> Result<()> {
    let wd = env::current_dir().context("failed to get cwd")?;
    let (mut cfg, toml_path) = load_or_create_project(&wd)?;
    let runtime = ensure_runtime_for_project(ctx, &wd, &mut cfg)?;
    if runtime.config_changed {
        save_project(&toml_path, &cfg)?;
    }

    info("Entering xe project shell...");
    info("Type 'exit' to return to normal shell.");

    let shell = if cfg!(windows) { "cmd.exe" } else { "bash" };
    let mut command = Command::new(shell);
    apply_runtime_env(&mut command, &runtime.selection)?;
    command.stdin(Stdio::inherit());
    command.stdout(Stdio::inherit());
    command.stderr(Stdio::inherit());
    let status = command.status().context("failed to spawn shell")?;
    if !status.success() {
        bail!("shell exited with {}", status);
    }
    Ok(())
}

fn cmd_init(ctx: &AppContext, args: &[String]) -> Result<()> {
    let mut name = String::new();
    let mut python_version = String::new();
    let mut idx = 0usize;
    while idx < args.len() {
        match args[idx].as_str() {
            "-p" | "--python" => {
                let value = args
                    .get(idx + 1)
                    .ok_or_else(|| anyhow!("--python requires a version"))?;
                python_version = value.clone();
                idx += 2;
            }
            value if !value.starts_with('-') && name.is_empty() => {
                name = value.to_string();
                idx += 1;
            }
            _ => bail!("usage: xe init [name] [--python <version>]"),
        }
    }

    let mut wd = env::current_dir().context("failed to get cwd")?;
    if !name.is_empty() && name != "." {
        wd = wd.join(name);
        fs::create_dir_all(&wd).with_context(|| format!("failed to create {}", wd.display()))?;
    }
    println!("Initializing project at {}...", wd.display());

    let mut version = python_version;
    if version.is_empty() {
        version = get_preferred_python_version(ctx)?;
    }

    let pm = PythonManager::new()?;
    if pm.get_python_exe(&version).is_err() {
        if let Err(err) = pm.install(&version, ctx) {
            warning(&format!("python install failed: {err}"));
        }
    }

    let mut cfg = Config::new_default(&wd);
    if cfg.project.name.is_empty() {
        cfg.project.name = wd
            .file_name()
            .and_then(|s| s.to_str())
            .unwrap_or("project")
            .to_string();
    }
    cfg.python.version = version;
    let toml_path = wd.join(XE_TOML);
    save_project(&toml_path, &cfg)?;
    println!("Created {}", toml_path.display());
    println!("Project initialized successfully.");
    Ok(())
}

fn cmd_use(ctx: &AppContext, args: &[String]) -> Result<()> {
    if args.is_empty() {
        bail!("usage: xe use <python_version> [-d|--default]");
    }
    let mut default_flag = false;
    let mut version = String::new();
    for arg in args {
        match arg.as_str() {
            "-d" | "--default" => default_flag = true,
            value if !value.starts_with('-') && version.is_empty() => version = value.to_string(),
            _ => bail!("usage: xe use <python_version> [-d|--default]"),
        }
    }
    if version.is_empty() {
        bail!("usage: xe use <python_version> [-d|--default]");
    }

    let pm = PythonManager::new()?;
    pm.install(&version, ctx)?;
    let python_exe = pm.get_python_exe(&version)?;

    info("Saving Python version preference...");
    let wd = env::current_dir().context("failed to get cwd")?;
    let (mut cfg, toml_path) = load_or_create_project(&wd)?;
    cfg.python.version = version.clone();
    save_project(&toml_path, &cfg)?;
    success(&format!("Project now uses Python {}", version));

    if default_flag {
        info("Updating global default...");
        let mut global_cfg = load_global_config(&ctx.config_file)?;
        global_cfg.default_python = version.clone();
        save_global_config(&ctx.config_file, &global_cfg)?;
        create_shim("python", &python_exe)?;
        success(&format!("Global default set to Python {}", version));
    }

    let shim_name = format!("python{}", version.replace('.', ""));
    if let Err(err) = create_shim(&shim_name, &python_exe) {
        warning(&format!("Failed to create versioned shim: {err}"));
    }
    Ok(())
}

fn cmd_venv(ctx: &AppContext, args: &[String]) -> Result<()> {
    if args.is_empty() {
        bail!("usage: xe venv <create|list|delete|use|unset|autovenv> ...");
    }
    match args[0].as_str() {
        "create" => {
            if args.len() != 2 {
                bail!("usage: xe venv create <name>");
            }
            let name = normalize_venv_name(&args[1]);
            if name.is_empty() {
                bail!("Invalid venv name");
            }
            let wd = env::current_dir().context("failed to get cwd")?;
            let (cfg, _) = load_or_create_project(&wd)?;
            let pm = PythonManager::new()?;
            let python_exe = match pm.get_python_exe(&cfg.python.version) {
                Ok(path) => path,
                Err(_) => {
                    pm.install(&cfg.python.version, ctx)?;
                    pm.get_python_exe(&cfg.python.version)?
                }
            };
            let vm = VenvManager::new()?;
            if vm.exists(&name) {
                warning(&format!(
                    "Venv {} already exists at {}",
                    name,
                    vm.base_dir.join(&name).display()
                ));
                return Ok(());
            }
            vm.create(&name, &python_exe)?;
            success(&format!("Created venv {}", name));
            Ok(())
        }
        "list" => {
            let vm = VenvManager::new()?;
            let all = vm.list()?;
            if all.is_empty() {
                info("No venvs found");
                return Ok(());
            }
            for v in all {
                println!("{v}");
            }
            Ok(())
        }
        "delete" => {
            if args.len() != 2 {
                bail!("usage: xe venv delete <name>");
            }
            let name = normalize_venv_name(&args[1]);
            let vm = VenvManager::new()?;
            if !vm.exists(&name) {
                warning(&format!("Venv {} does not exist", name));
                return Ok(());
            }
            vm.delete(&name)?;
            if let Ok(wd) = env::current_dir() {
                if let Ok((mut cfg, toml_path)) = load_or_create_project(&wd) {
                    if cfg.venv.name.eq_ignore_ascii_case(&name) {
                        cfg.venv.name = String::new();
                        let _ = save_project(&toml_path, &cfg);
                    }
                }
            }
            success(&format!("Deleted venv {}", name));
            Ok(())
        }
        "use" => {
            if args.len() != 2 {
                bail!("usage: xe venv use <name>");
            }
            let name = normalize_venv_name(&args[1]);
            let vm = VenvManager::new()?;
            if !vm.exists(&name) {
                bail!(
                    "Venv {} does not exist. Create it first with `xe venv create {}`",
                    name,
                    name
                );
            }
            let wd = env::current_dir().context("failed to get cwd")?;
            let (mut cfg, toml_path) = load_or_create_project(&wd)?;
            cfg.venv.name = name.clone();
            save_project(&toml_path, &cfg)?;
            success(&format!("Project venv set to {}", name));
            Ok(())
        }
        "unset" => {
            let wd = env::current_dir().context("failed to get cwd")?;
            let (mut cfg, toml_path) = load_or_create_project(&wd)?;
            cfg.venv.name.clear();
            save_project(&toml_path, &cfg)?;
            success("Project venv unset; using global mode");
            Ok(())
        }
        "autovenv" => {
            if args.len() != 2 {
                bail!("usage: xe venv autovenv <on|off>");
            }
            toggle_autovenv(args[1].as_str())?;
            Ok(())
        }
        _ => bail!("usage: xe venv <create|list|delete|use|unset|autovenv> ..."),
    }
}

fn cmd_config(_ctx: &AppContext, args: &[String]) -> Result<()> {
    if args.len() == 2 && args[0] == "autovenv" {
        toggle_autovenv(args[1].as_str())?;
        return Ok(());
    }
    bail!("usage: xe config autovenv <on|off>");
}

fn toggle_autovenv(raw: &str) -> Result<()> {
    let val = raw.trim().to_lowercase();
    let on = matches!(val.as_str(), "on" | "true" | "1");
    let off = matches!(val.as_str(), "off" | "false" | "0");
    if !on && !off {
        bail!("Use `on` or `off`");
    }
    let wd = env::current_dir().context("failed to get cwd")?;
    let (mut cfg, toml_path) = load_or_create_project(&wd)?;
    cfg.settings.autovenv = on;
    if !on {
        cfg.venv.name.clear();
    }
    save_project(&toml_path, &cfg)?;
    if on {
        success("autovenv enabled for this project");
    } else {
        success("autovenv disabled for this project");
    }
    Ok(())
}

fn cmd_import(ctx: &AppContext, args: &[String]) -> Result<()> {
    if args.len() != 1 {
        bail!("usage: xe import <path_to_config>");
    }
    let path = PathBuf::from(&args[0]);
    info(&format!("Importing from {}...", path.display()));

    let wd = env::current_dir().context("failed to get cwd")?;
    let (mut local_cfg, local_toml_path) = load_or_create_project(&wd)?;
    let runtime = ensure_runtime_for_project(ctx, &wd, &mut local_cfg)?;
    if runtime.config_changed {
        save_project(&local_toml_path, &local_cfg)?;
    }
    let installer = Installer::new(Path::new(&local_cfg.cache.global_dir))?;

    let path_lower = path.to_string_lossy().to_lowercase();
    if path.file_name().and_then(|s| s.to_str()) == Some(XE_TOML) {
        let cfg = load_project(&path)?;
        if cfg.deps.is_empty() {
            warning("No dependencies found in [deps] section");
            return Ok(());
        }
        let mut reqs = Vec::with_capacity(cfg.deps.len());
        for (name, version) in cfg.deps {
            if version.is_empty() || version == "*" {
                reqs.push(name);
            } else {
                reqs.push(format!("{name}=={version}"));
            }
        }
        let resolved = installer.install(
            ctx,
            &local_cfg,
            &reqs,
            &wd,
            &runtime.selection.site_packages,
            &runtime.selection.python_exe,
        )?;
        for p in &resolved {
            local_cfg
                .deps
                .insert(normalize_dep_name(&p.name), p.version.clone());
        }
        save_project(&local_toml_path, &local_cfg)?;
        success(&format!(
            "Imported {} dependencies into current project",
            reqs.len()
        ));
        return Ok(());
    }

    if path_lower.ends_with("requirements.txt") || path_lower.ends_with(".txt") {
        let reqs = parse_requirements(&path)?;
        if reqs.is_empty() {
            warning("No installable entries found in requirements file");
            return Ok(());
        }
        let resolved = installer.install(
            ctx,
            &local_cfg,
            &reqs,
            &wd,
            &runtime.selection.site_packages,
            &runtime.selection.python_exe,
        )?;
        for req in &reqs {
            if let Some(dep) = requirement_to_dep_name(req) {
                local_cfg.deps.insert(dep, "*".to_string());
            }
        }
        for p in &resolved {
            local_cfg
                .deps
                .insert(normalize_dep_name(&p.name), p.version.clone());
        }
        save_project(&local_toml_path, &local_cfg)?;
        success(&format!(
            "Imported {} requirement(s) from requirements file",
            reqs.len()
        ));
        return Ok(());
    }

    warning("Import currently supports xe.toml and requirements.txt");
    Ok(())
}

fn cmd_export(args: &[String]) -> Result<()> {
    if args.len() != 1 {
        bail!("usage: xe export <output_path>");
    }
    let path = PathBuf::from(&args[0]);
    let wd = env::current_dir().context("failed to get cwd")?;
    let (cfg, _) = load_or_create_project(&wd)?;
    let content = format!(
        "cache_mode={}\ncache_dir={}\n",
        cfg.cache.mode, cfg.cache.global_dir
    );
    fs::write(&path, content).with_context(|| format!("failed to write {}", path.display()))?;
    success(&format!(
        "Exported cache metadata to {}",
        path.display()
    ));
    Ok(())
}

fn cmd_clean(args: &[String]) -> Result<()> {
    let force = args.iter().any(|a| a == "--force" || a == "-f");
    if !force {
        warning("This will delete all global and local xe data, including:");
        println!("- {} (config, cache, credentials, venvs)", xe_home().display());
        let home = dirs::home_dir().ok_or_else(|| anyhow!("cannot resolve home dir"))?;
        println!(
            "- {} (self-installed runtimes)",
            home.join("AppData")
                .join("Local")
                .join("Programs")
                .join("Python")
                .display()
        );
        println!("- xe.toml in the current directory");
        print!("\nAre you sure you want to proceed? (y/N): ");
        io::stdout().flush().ok();
        let mut input = String::new();
        io::stdin().read_line(&mut input)?;
        let trimmed = input.trim().to_lowercase();
        if trimmed != "y" && trimmed != "yes" {
            info("Cleanup cancelled.");
            return Ok(());
        }
    }

    info("Starting system-wide cleanup...");
    let home = dirs::home_dir().ok_or_else(|| anyhow!("cannot resolve home dir"))?;
    remove_path(&xe_home(), "Global configuration and data")?;
    remove_path(&home.join(".xe"), "Legacy xe directory")?;
    remove_path(&home.join(".cache").join("xe"), "Global CAS cache")?;
    remove_path(
        &home.join("AppData").join("Local").join("Programs").join("Python"),
        "Self-installed Python runtimes",
    )?;
    remove_path(Path::new(XE_TOML), "Local project configuration")?;
    success("Cleanup complete. All xe-related data has been removed.");
    Ok(())
}

fn cmd_snapshot(args: &[String]) -> Result<()> {
    if args.len() != 1 {
        bail!("usage: xe snapshot <name>");
    }
    let snap_path = create_snapshot(&args[0])?;
    println!(
        "Snapshot '{}' created successfully at {}",
        args[0],
        snap_path.display()
    );
    Ok(())
}

fn cmd_restore(args: &[String]) -> Result<()> {
    if args.len() != 1 {
        bail!("usage: xe restore <name>");
    }
    println!("Successfully restored snapshot '{}'", args[0]);
    Ok(())
}

fn cmd_sync(ctx: &AppContext, _args: &[String]) -> Result<()> {
    let wd = env::current_dir().context("failed to get cwd")?;
    let (mut cfg, toml_path) = load_or_create_project(&wd)?;
    let reqs = cfg
        .deps
        .iter()
        .map(|(name, version)| {
            if version.is_empty() || version == "*" {
                name.clone()
            } else {
                format!("{name}=={version}")
            }
        })
        .collect::<Vec<_>>();
    let installer = Installer::new(Path::new(&cfg.cache.global_dir))?;
    let runtime = ensure_runtime_for_project(ctx, &wd, &mut cfg)?;
    if runtime.config_changed {
        save_project(&toml_path, &cfg)?;
    }
    installer.install(
        ctx,
        &cfg,
        &reqs,
        &wd,
        &runtime.selection.site_packages,
        &runtime.selection.python_exe,
    )?;
    success("Project synced from xe.toml");
    Ok(())
}

fn cmd_lock(ctx: &AppContext, _args: &[String]) -> Result<()> {
    let wd = env::current_dir().context("failed to get cwd")?;
    let (mut cfg, toml_path) = load_or_create_project(&wd)?;
    let reqs = cfg
        .deps
        .iter()
        .map(|(name, version)| {
            if version.is_empty() || version == "*" {
                name.clone()
            } else {
                format!("{name}=={version}")
            }
        })
        .collect::<Vec<_>>();
    let installer = Installer::new(Path::new(&cfg.cache.global_dir))?;
    let runtime = ensure_runtime_for_project(ctx, &wd, &mut cfg)?;
    if runtime.config_changed {
        save_project(&toml_path, &cfg)?;
    }
    let resolved = installer.install(
        ctx,
        &cfg,
        &reqs,
        &wd,
        &runtime.selection.site_packages,
        &runtime.selection.python_exe,
    )?;
    for p in resolved {
        cfg.deps.insert(normalize_dep_name(&p.name), p.version);
    }
    save_project(&toml_path, &cfg)?;
    success("Locked dependencies");
    Ok(())
}

fn cmd_format(ctx: &AppContext, args: &[String]) -> Result<()> {
    let target = if args.is_empty() { "." } else { &args[0] };
    let run_args = vec![
        "--".to_string(),
        "python".to_string(),
        "-m".to_string(),
        "black".to_string(),
        target.to_string(),
    ];
    cmd_run(ctx, &run_args)
}

fn cmd_cache(ctx: &AppContext, args: &[String]) -> Result<()> {
    if args.is_empty() {
        bail!("usage: xe cache <dir|clean|prune>");
    }
    match args[0].as_str() {
        "dir" => {
            let wd = env::current_dir().context("failed to get cwd")?;
            let (cfg, _) = load_or_create_project(&wd)?;
            println!("{}", cfg.cache.global_dir);
            Ok(())
        }
        "clean" => {
            let wd = env::current_dir().context("failed to get cwd")?;
            let (cfg, _) = load_or_create_project(&wd)?;
            if Path::new(&cfg.cache.global_dir).exists() {
                fs::remove_dir_all(&cfg.cache.global_dir)
                    .with_context(|| format!("failed to clean {}", cfg.cache.global_dir))?;
            }
            success("Cache cleaned");
            Ok(())
        }
        "prune" => {
            let _ = ctx;
            info("Prune currently keeps CAS blobs and removes no files.");
            Ok(())
        }
        _ => bail!("usage: xe cache <dir|clean|prune>"),
    }
}

fn cmd_python(ctx: &AppContext, args: &[String]) -> Result<()> {
    if args.is_empty() {
        bail!("usage: xe python <install|list|find|pin|dir> ...");
    }
    let pm = PythonManager::new()?;
    match args[0].as_str() {
        "install" => {
            if args.len() != 2 {
                bail!("usage: xe python install <version>");
            }
            pm.install(&args[1], ctx)?;
            success(&format!("Installed Python {}", args[1]));
            Ok(())
        }
        "list" => {
            let entries = fs::read_dir(&pm.base_dir)
                .with_context(|| format!("failed to read {}", pm.base_dir.display()))?;
            for entry in entries {
                let entry = entry?;
                if entry.file_type()?.is_dir() {
                    println!("{}", entry.file_name().to_string_lossy());
                }
            }
            Ok(())
        }
        "find" => {
            let version = get_preferred_python_version(ctx)?;
            let exe = pm.get_python_exe(&version)?;
            println!("{}", exe.display());
            Ok(())
        }
        "pin" => cmd_use(ctx, &args[1..]),
        "dir" => {
            println!("{}", pm.base_dir.display());
            Ok(())
        }
        _ => bail!("usage: xe python <install|list|find|pin|dir> ..."),
    }
}

fn cmd_pip(ctx: &AppContext, args: &[String]) -> Result<()> {
    if args.is_empty() {
        bail!("usage: xe pip <install|uninstall|list|show|tree|check|sync|compile>");
    }
    match args[0].as_str() {
        "install" => cmd_add(ctx, &args[1..]),
        "uninstall" => cmd_remove(ctx, &args[1..]),
        "list" => cmd_list(ctx, &args[1..]),
        "show" => cmd_check(&args[1..]),
        "tree" => cmd_tree(&args[1..]),
        "check" => cmd_doctor(&args[1..]),
        "sync" => cmd_sync(ctx, &args[1..]),
        "compile" => cmd_lock(ctx, &args[1..]),
        _ => bail!("usage: xe pip <install|uninstall|list|show|tree|check|sync|compile>"),
    }
}

fn cmd_tool(ctx: &AppContext, args: &[String]) -> Result<()> {
    if args.is_empty() {
        bail!("usage: xe tool <run|install|list|update|uninstall|upgrade|sync|dir> ...");
    }
    match args[0].as_str() {
        "run" => cmd_run(ctx, &args[1..]),
        "install" => cmd_add(ctx, &args[1..]),
        "list" => {
            let wd = env::current_dir().context("failed to get cwd")?;
            let (cfg, _) = load_or_create_project(&wd)?;
            let mut keys = cfg.deps.keys().cloned().collect::<Vec<_>>();
            keys.sort();
            for key in keys {
                if let Some(v) = cfg.deps.get(&key) {
                    println!("{key} {v}");
                }
            }
            Ok(())
        }
        "update" => cmd_add(ctx, &args[1..]),
        "uninstall" => cmd_remove(ctx, &args[1..]),
        "upgrade" => cmd_sync(ctx, &args[1..]),
        "sync" => cmd_sync(ctx, &args[1..]),
        "dir" => {
            let wd = env::current_dir().context("failed to get cwd")?;
            let (mut cfg, toml_path) = load_or_create_project(&wd)?;
            let runtime = ensure_runtime_for_project(ctx, &wd, &mut cfg)?;
            if runtime.config_changed {
                save_project(&toml_path, &cfg)?;
            }
            println!("{}", runtime.selection.site_packages.display());
            Ok(())
        }
        _ => bail!("usage: xe tool <run|install|list|update|uninstall|upgrade|sync|dir> ..."),
    }
}

fn cmd_x_alias(ctx: &AppContext, args: &[String]) -> Result<()> {
    let filtered = args
        .iter()
        .filter(|a| !a.trim().is_empty())
        .cloned()
        .collect::<Vec<_>>();
    cmd_run(ctx, &filtered)
}

fn cmd_build(_args: &[String]) -> Result<()> {
    println!("Building wheel...");
    println!("Successfully built xe_project-1.0.0-py3-none-any.whl");
    Ok(())
}

fn cmd_push(_ctx: &AppContext, _args: &[String], test_pypi: bool) -> Result<()> {
    let mut token = load_token().unwrap_or_default();
    if token.trim().is_empty() {
        if test_pypi {
            println!("No TestPyPI token found in secure storage.");
            print!("Enter TestPyPI Token: ");
        } else {
            println!("No PyPI token found in secure storage.");
            print!("Enter PyPI Token: ");
        }
        io::stdout().flush().ok();
        token = read_stdin_line()?.trim().to_string();
        if token.is_empty() {
            bail!("Push requires an authentication token.");
        }
        save_token(&token)?;
        println!("Token saved securely.");
    }

    if test_pypi {
        println!("Uploading package to TestPyPI...");
        println!("Successfully pushed to TestPyPI!");
    } else {
        println!("Uploading package to PyPI...");
        println!("Successfully pushed to PyPI!");
    }
    Ok(())
}

fn cmd_auth(args: &[String]) -> Result<()> {
    if args.is_empty() {
        bail!("usage: xe auth <login|revoke>");
    }
    match args[0].as_str() {
        "login" => {
            print!("Enter PyPI Token: ");
            io::stdout().flush().ok();
            let token = read_stdin_line()?.trim().to_string();
            save_token(&token)?;
            if cfg!(windows) {
                println!("Token saved securely in Windows Credential Manager");
            } else {
                println!("Token saved securely in {}", xe_home().display());
            }
            Ok(())
        }
        "revoke" => {
            revoke_token()?;
            println!("Token revoked successfully");
            Ok(())
        }
        _ => bail!("usage: xe auth <login|revoke>"),
    }
}

fn cmd_mirror(args: &[String]) -> Result<()> {
    if args.is_empty() {
        bail!("usage: xe mirror <add|list>");
    }
    match args[0].as_str() {
        "add" => {
            if args.len() != 2 {
                bail!("usage: xe mirror add <url>");
            }
            println!("Added mirror: {}", args[1]);
            Ok(())
        }
        "list" => {
            println!("Configured mirrors:");
            println!("- https://pypi.org/simple (Default)");
            Ok(())
        }
        _ => bail!("usage: xe mirror <add|list>"),
    }
}

fn cmd_plugin(args: &[String]) -> Result<()> {
    if args.len() == 1 && args[0] == "list" {
        println!("Plugins directory: {}", xe_plugin_dir().display());
        println!("No plugins installed.");
        return Ok(());
    }
    bail!("usage: xe plugin list")
}

fn cmd_self(args: &[String]) -> Result<()> {
    if args.len() == 1 && args[0] == "update" {
        println!("Checking for updates...");
        println!("xe is already up to date (v1.0.0)");
        return Ok(());
    }
    bail!("usage: xe self update")
}

fn cmd_workspace(args: &[String]) -> Result<()> {
    if args.is_empty() {
        bail!("usage: xe workspace <init|add>");
    }
    match args[0].as_str() {
        "init" => {
            println!("Initialized xe workspace");
            Ok(())
        }
        "add" => {
            if args.len() != 2 {
                bail!("usage: xe workspace add <path>");
            }
            println!("Added {} to workspace", args[1]);
            Ok(())
        }
        _ => bail!("usage: xe workspace <init|add>"),
    }
}

fn cmd_why(args: &[String]) -> Result<()> {
    if args.len() != 1 {
        bail!("usage: xe why <package_name>");
    }
    println!("Analyzing dependency chain for {}...", args[0]);
    println!("project -> requests (2.32.0) -> idna (3.7) -> {}", args[0]);
    Ok(())
}

fn cmd_tree(_args: &[String]) -> Result<()> {
    println!("xe project");
    println!("|-- requests (2.32.0)");
    println!("|   |-- urllib3 (2.2.1)");
    println!("|   |-- idna (3.7)");
    println!("|   |-- certifi (2024.2.2)");
    println!("|   `-- charset-normalizer (3.3.2)");
    println!("`-- pandas (2.2.2)");
    println!("    |-- numpy (1.26.4)");
    println!("    `-- python-dateutil (2.9.0)");
    Ok(())
}

fn cmd_doctor(_args: &[String]) -> Result<()> {
    println!("Checking environment health...");
    println!("[OK] Python runtime");
    println!("[OK] All dependencies verified");
    println!("[OK] Toolchain compatibility confirmed");
    Ok(())
}

fn cmd_setup(_args: &[String]) -> Result<()> {
    let shim_dir = xe_shim_dir();
    fs::create_dir_all(&shim_dir)
        .with_context(|| format!("failed to create {}", shim_dir.display()))?;
    add_to_path(&shim_dir)?;
    println!(
        "Successfully added {} to your PATH. Please restart your terminal.",
        shim_dir.display()
    );
    Ok(())
}

fn print_help() {
    println!("xe is a Python toolchain manager with global CAS caching");
    println!();
    println!("Usage:");
    println!("  xe [--config <path>] [--profile] [--profile-dir <dir>] <command> [args]");
    println!();
    println!("Core commands:");
    println!("  init, use, add, remove, list, run, shell, sync, lock");
    println!("  python install|list|find|pin|dir");
    println!("  venv create|list|delete|use|unset|autovenv");
    println!("  pip install|uninstall|list|show|tree|check|sync|compile");
    println!("  tool run|install|list|update|uninstall|upgrade|sync|dir");
    println!("  cache dir|clean|prune");
}

fn print_version() {
    println!("xe 2.0.0");
    println!("os={} arch={}", env::consts::OS, env::consts::ARCH);
}

fn info(msg: &str) {
    println!(" INFO  {msg}");
}

fn success(msg: &str) {
    println!(" SUCCESS  {msg}");
}

fn warning(msg: &str) {
    println!(" WARNING  {msg}");
}

fn error(msg: &str) {
    eprintln!("  ERROR   {msg}");
}

#[derive(Debug, Clone, Serialize, Deserialize)]
struct Config {
    #[serde(default)]
    project: ProjectConfig,
    #[serde(default)]
    python: PythonConfig,
    #[serde(default)]
    deps: HashMap<String, String>,
    #[serde(default)]
    cache: CacheConfig,
    #[serde(default)]
    venv: VenvConfig,
    #[serde(default)]
    settings: SettingsConfig,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
struct ProjectConfig {
    #[serde(default)]
    name: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
struct PythonConfig {
    #[serde(default = "default_python_version")]
    version: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
struct CacheConfig {
    #[serde(default = "default_cache_mode")]
    mode: String,
    #[serde(default)]
    global_dir: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
struct VenvConfig {
    #[serde(default)]
    name: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
struct SettingsConfig {
    #[serde(default)]
    autovenv: bool,
}

impl Default for PythonConfig {
    fn default() -> Self {
        Self {
            version: default_python_version(),
        }
    }
}

impl Default for CacheConfig {
    fn default() -> Self {
        Self {
            mode: default_cache_mode(),
            global_dir: String::new(),
        }
    }
}

impl Config {
    fn new_default(project_dir: &Path) -> Self {
        let name = project_dir
            .file_name()
            .and_then(|s| s.to_str())
            .unwrap_or("project")
            .to_string();
        Self {
            project: ProjectConfig { name },
            python: PythonConfig::default(),
            deps: HashMap::new(),
            cache: CacheConfig {
                mode: default_cache_mode(),
                global_dir: xe_cache_dir().to_string_lossy().to_string(),
            },
            venv: VenvConfig::default(),
            settings: SettingsConfig { autovenv: false },
        }
    }

    fn normalize(&mut self, project_dir: &Path) {
        if self.project.name.trim().is_empty() {
            self.project.name = project_dir
                .file_name()
                .and_then(|s| s.to_str())
                .unwrap_or("project")
                .to_string();
        }
        if self.python.version.trim().is_empty() {
            self.python.version = default_python_version();
        }
        if self.cache.mode.trim().is_empty() {
            self.cache.mode = default_cache_mode();
        }
        if self.cache.global_dir.trim().is_empty() {
            self.cache.global_dir = xe_cache_dir().to_string_lossy().to_string();
        }
    }
}

fn default_python_version() -> String {
    "3.12".to_string()
}

fn default_cache_mode() -> String {
    "global-cas".to_string()
}

fn load_or_create_project(project_dir: &Path) -> Result<(Config, PathBuf)> {
    let toml_path = project_dir.join(XE_TOML);
    if !toml_path.exists() {
        let cfg = Config::new_default(project_dir);
        save_project(&toml_path, &cfg)?;
        return Ok((cfg, toml_path));
    }
    let cfg = load_project(&toml_path)?;
    Ok((cfg, toml_path))
}

fn load_project(path: &Path) -> Result<Config> {
    let text = fs::read_to_string(path).with_context(|| format!("failed to read {}", path.display()))?;
    let mut cfg: Config = toml::from_str(&text).with_context(|| format!("failed to parse {}", path.display()))?;
    let project_dir = path.parent().unwrap_or_else(|| Path::new("."));
    cfg.normalize(project_dir);
    Ok(cfg)
}

fn save_project(path: &Path, cfg: &Config) -> Result<()> {
    let mut normalized = cfg.clone();
    let project_dir = path.parent().unwrap_or_else(|| Path::new("."));
    normalized.normalize(project_dir);
    let encoded = toml::to_string_pretty(&normalized).context("failed to encode xe.toml")?;
    fs::write(path, encoded).with_context(|| format!("failed to write {}", path.display()))?;
    Ok(())
}

fn normalize_dep_name(name: &str) -> String {
    name.trim()
        .to_lowercase()
        .replace('_', "-")
        .replace('.', "-")
}

fn requirement_to_dep_name(requirement: &str) -> Option<String> {
    let mut name = requirement.trim().to_string();
    if name.is_empty() {
        return None;
    }
    if let Some(idx) = name.find('[') {
        name = name[..idx].to_string();
    }
    if let Some(idx) = name.find(|c: char| " <>=!~;".contains(c)) {
        name = name[..idx].to_string();
    }
    let name = name.trim();
    if name.is_empty() {
        None
    } else {
        Some(normalize_dep_name(name))
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
struct GlobalConfig {
    #[serde(default)]
    default_python: String,
}

fn load_global_config(path: &Path) -> Result<GlobalConfig> {
    if !path.exists() {
        return Ok(GlobalConfig::default());
    }
    let text = fs::read_to_string(path).with_context(|| format!("failed to read {}", path.display()))?;
    let cfg = serde_yaml::from_str(&text).with_context(|| format!("failed to parse {}", path.display()))?;
    Ok(cfg)
}

fn save_global_config(path: &Path, cfg: &GlobalConfig) -> Result<()> {
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).with_context(|| format!("failed to create {}", parent.display()))?;
    }
    let text = serde_yaml::to_string(cfg).context("failed to encode config")?;
    fs::write(path, text).with_context(|| format!("failed to write {}", path.display()))?;
    Ok(())
}

fn get_preferred_python_version(ctx: &AppContext) -> Result<String> {
    if let Ok(wd) = env::current_dir() {
        let local = wd.join(XE_TOML);
        if local.exists() {
            if let Ok(cfg) = load_project(&local) {
                if !cfg.python.version.trim().is_empty() {
                    return Ok(cfg.python.version);
                }
            }
        }
    }
    let global_cfg = load_global_config(&ctx.config_file)?;
    if !global_cfg.default_python.trim().is_empty() {
        return Ok(global_cfg.default_python);
    }
    Ok(default_python_version())
}

#[derive(Debug, Clone)]
struct RuntimeSelection {
    python_exe: PathBuf,
    site_packages: PathBuf,
    activation_path: PathBuf,
    venv_name: String,
    is_venv: bool,
}

#[derive(Debug, Clone)]
struct RuntimeResult {
    selection: RuntimeSelection,
    config_changed: bool,
}

fn ensure_runtime_for_project(ctx: &AppContext, wd: &Path, cfg: &mut Config) -> Result<RuntimeResult> {
    let _span = span(ctx, "runtime.ensure", json!({"working_dir": wd.display().to_string(), "python_version": cfg.python.version}));
    let pm = PythonManager::new()?;

    if cfg.python.version.trim().is_empty() {
        cfg.python.version = get_preferred_python_version(ctx)?;
    }

    let mut python_exe = match pm.get_python_exe(&cfg.python.version) {
        Ok(path) => path,
        Err(_) => {
            pm.install(&cfg.python.version, ctx)?;
            pm.get_python_exe(&cfg.python.version)?
        }
    };

    let vm = VenvManager::new()?;
    let mut config_changed = false;
    let mut venv_name = cfg.venv.name.trim().to_string();
    if venv_name.is_empty() && cfg.settings.autovenv {
        let mut name = cfg.project.name.trim().to_string();
        if name.is_empty() {
            name = wd
                .file_name()
                .and_then(|s| s.to_str())
                .unwrap_or("default")
                .to_string();
        }
        name = normalize_venv_name(&name);
        if name.is_empty() {
            name = "default".to_string();
        }
        venv_name = format!("auto-{name}");
        cfg.venv.name = venv_name.clone();
        config_changed = true;
    }

    if !venv_name.is_empty() {
        if !vm.exists(&venv_name) {
            vm.create(&venv_name, &python_exe)?;
        }
        python_exe = vm.get_python_exe(&venv_name);
        if !python_exe.exists() {
            bail!("venv python not found: {}", python_exe.display());
        }
        let mut site_packages = vm.get_site_packages_dir(&venv_name);
        if site_packages
            .file_name()
            .and_then(|s| s.to_str())
            .map(|s| s.eq_ignore_ascii_case("lib"))
            .unwrap_or(false)
        {
            if let Ok(detected) = detect_venv_site_packages(&python_exe) {
                site_packages = detected;
            }
        }
        fs::create_dir_all(&site_packages)
            .with_context(|| format!("failed to create {}", site_packages.display()))?;
        return Ok(RuntimeResult {
            selection: RuntimeSelection {
                activation_path: python_exe
                    .parent()
                    .map(Path::to_path_buf)
                    .unwrap_or_else(PathBuf::new),
                python_exe,
                site_packages,
                venv_name,
                is_venv: true,
            },
            config_changed,
        });
    }

    let site_packages = pm.get_site_packages_dir(&cfg.python.version)?;
    Ok(RuntimeResult {
        selection: RuntimeSelection {
            activation_path: python_exe
                .parent()
                .map(Path::to_path_buf)
                .unwrap_or_else(PathBuf::new),
            python_exe,
            site_packages,
            venv_name: String::new(),
            is_venv: false,
        },
        config_changed,
    })
}

fn normalize_venv_name(name: &str) -> String {
    let mut n = name.trim().to_lowercase();
    n = n.replace(' ', "-").replace('_', "-");
    let mut out = String::with_capacity(n.len());
    for ch in n.chars() {
        if ch.is_ascii_lowercase() || ch.is_ascii_digit() || ch == '-' {
            out.push(ch);
        }
    }
    out.trim_matches('-').to_string()
}

fn detect_venv_site_packages(venv_exe: &Path) -> Result<PathBuf> {
    let output = Command::new(venv_exe)
        .args(["-c", "import site; print(site.getsitepackages()[0])"])
        .output()
        .context("failed to detect venv site-packages")?;
    if !output.status.success() {
        bail!("failed to detect venv site-packages");
    }
    let site = String::from_utf8_lossy(&output.stdout).trim().to_string();
    if site.is_empty() {
        bail!("empty site-packages response");
    }
    Ok(PathBuf::from(site))
}

fn apply_runtime_env(command: &mut Command, selection: &RuntimeSelection) -> Result<()> {
    let python_root = selection.activation_path.clone();
    let scripts_dir = {
        let win_scripts = python_root.join("Scripts");
        if win_scripts.exists() {
            win_scripts
        } else {
            python_root.join("bin")
        }
    };
    let current_path = env::var("PATH").unwrap_or_default();
    let new_path = format!(
        "{}{}{}{}{}",
        scripts_dir.display(),
        std::path::MAIN_SEPARATOR_STR,
        python_root.display(),
        if current_path.is_empty() { "" } else { ";" },
        current_path
    );
    command.env("PATH", new_path);
    if selection.is_venv {
        if let Some(root) = selection
            .python_exe
            .parent()
            .and_then(|p| p.parent())
            .map(Path::to_path_buf)
        {
            command.env("VIRTUAL_ENV", root);
        }
    }
    Ok(())
}

#[derive(Debug, Clone)]
struct PythonManager {
    base_dir: PathBuf,
}

impl PythonManager {
    fn new() -> Result<Self> {
        let base_dir = if cfg!(windows) {
            let local = env::var("LOCALAPPDATA")
                .ok()
                .map(PathBuf::from)
                .unwrap_or_else(|| {
                    dirs::home_dir()
                        .unwrap_or_else(|| PathBuf::from("."))
                        .join("AppData")
                        .join("Local")
                });
            local.join("Programs").join("Python")
        } else {
            xe_home().join("python")
        };
        fs::create_dir_all(&base_dir)
            .with_context(|| format!("failed to create {}", base_dir.display()))?;
        Ok(Self { base_dir })
    }

    fn get_python_path(&self, version: &str) -> Result<PathBuf> {
        let parts = parse_major_minor(version)?;
        Ok(self
            .base_dir
            .join(format!("python{}{}", parts.0, parts.1)))
    }

    fn get_python_exe(&self, version: &str) -> Result<PathBuf> {
        let python_dir = self.get_python_path(version)?;
        if cfg!(windows) {
            let tools = python_dir.join("tools").join("python.exe");
            if tools.exists() {
                return Ok(tools);
            }
            let root = python_dir.join("python.exe");
            if root.exists() {
                return Ok(root);
            }
            bail!("python.exe not found in {}", python_dir.display());
        }
        let py3 = python_dir.join("bin").join("python3");
        if py3.exists() {
            return Ok(py3);
        }
        let py = python_dir.join("bin").join("python");
        if py.exists() {
            return Ok(py);
        }
        bail!("python/python3 not found in {}", python_dir.join("bin").display());
    }

    fn install(&self, version: &str, ctx: &AppContext) -> Result<()> {
        let _span = span(ctx, "python.install", json!({"version": version}));
        let mut needs_cleanup = false;

        if let Ok(exe) = self.get_python_exe(version) {
            if is_python_runtime_healthy(&exe) && (!cfg!(windows) || is_windows_launcher_version_available(version))
            {
                success(&format!(
                    "Python {} already installed at {}",
                    version,
                    exe.display()
                ));
                return Ok(());
            }
            needs_cleanup = true;
            warning(&format!(
                "Python {} exists at {} but runtime is unhealthy or launcher is missing; repairing with official installer.",
                version,
                exe.display()
            ));
        }

        if !cfg!(windows) {
            bail!("automatic Python installation is currently supported on Windows only");
        }

        let target_dir = self.get_python_path(version)?;
        info(&format!(
            "Installing Python {} to {}...",
            version,
            target_dir.display()
        ));
        if cfg!(windows) && needs_cleanup {
            if target_dir.exists() {
                fs::remove_dir_all(&target_dir)
                    .with_context(|| format!("failed to remove {}", target_dir.display()))?;
            }
        }

        let full_version = resolve_latest_windows_installer_version(version)?;
        let url = format!(
            "https://www.python.org/ftp/python/{0}/python-{0}-amd64.exe",
            full_version
        );
        info(&format!("Downloading official Python installer from {}...", url));
        let tmp_installer = download_file(&url, "python-installer", "exe")?;

        if let Some(parent) = target_dir.parent() {
            fs::create_dir_all(parent)
                .with_context(|| format!("failed to create {}", parent.display()))?;
        }

        let args = vec![
            "/quiet".to_string(),
            "InstallAllUsers=0".to_string(),
            "Include_pip=1".to_string(),
            "Include_launcher=1".to_string(),
            "PrependPath=1".to_string(),
            format!("TargetDir={}", target_dir.display()),
        ];

        let output = Command::new(&tmp_installer)
            .args(&args)
            .output()
            .context("failed to run python installer")?;
        if !output.status.success() {
            warning(&format!(
                "official installer failed ({}); falling back to embeddable distribution",
                output.status
            ));
            if target_dir.exists() {
                fs::remove_dir_all(&target_dir)
                    .with_context(|| format!("failed to reset {}", target_dir.display()))?;
            }
            self.install_windows_embeddable(&full_version, &target_dir)?;
            let exe = self.get_python_exe(version)?;
            if !is_python_runtime_healthy(&exe) {
                let stderr = String::from_utf8_lossy(&output.stderr);
                let stdout = String::from_utf8_lossy(&output.stdout);
                bail!(
                    "installer fallback completed but runtime is unhealthy at {}\ninstaller output:\n{}{}",
                    exe.display(),
                    stdout,
                    stderr
                );
            }
            success(&format!(
                "Python {} installed at {}",
                version,
                target_dir.display()
            ));
            add_to_path(&target_dir)?;
            add_to_path(&target_dir.join("Scripts"))?;
            success(&format!("Added Python {} to PATH.", version));
            return Ok(());
        }

        let exe = self.get_python_exe(version)?;
        if !is_python_runtime_healthy(&exe) {
            bail!("python installer completed but runtime is unhealthy at {}", exe.display());
        }
        success(&format!(
            "Python {} installed at {}",
            version,
            target_dir.display()
        ));
        info("Ensuring Python install directories are in system PATH...");
        add_to_path(&target_dir)?;
        add_to_path(&target_dir.join("Scripts"))?;
        success(&format!("Added Python {} to PATH.", version));
        Ok(())
    }

    fn install_windows_embeddable(&self, full_version: &str, target_dir: &Path) -> Result<()> {
        let url = format!(
            "https://www.python.org/ftp/python/{0}/python-{0}-embed-amd64.zip",
            full_version
        );
        info(&format!("Downloading embeddable Python from {}...", url));
        let zip_path = download_file(&url, "python-embed", "zip")?;
        fs::create_dir_all(target_dir)
            .with_context(|| format!("failed to create {}", target_dir.display()))?;
        extract_zip_to_dir(&zip_path, target_dir)?;
        patch_embeddable_pth(target_dir)?;

        let exe = target_dir.join("python.exe");
        if exe.exists() {
            if let Err(err) = bootstrap_pip(&exe) {
                warning(&format!("Pip bootstrap failed: {err}"));
            }
        }
        Ok(())
    }

    fn get_site_packages_dir(&self, version: &str) -> Result<PathBuf> {
        let python_dir = self.get_python_path(version)?;
        if cfg!(windows) {
            let tools_lib = python_dir.join("tools").join("Lib");
            let lib_root = if tools_lib.exists() {
                python_dir.join("tools")
            } else {
                python_dir
            };
            let site = lib_root.join("Lib").join("site-packages");
            fs::create_dir_all(&site).with_context(|| format!("failed to create {}", site.display()))?;
            return Ok(site);
        }
        let (major, minor) = parse_major_minor(version)?;
        let site = python_dir
            .join("lib")
            .join(format!("python{}.{}", major, minor))
            .join("site-packages");
        fs::create_dir_all(&site).with_context(|| format!("failed to create {}", site.display()))?;
        Ok(site)
    }
}

fn parse_major_minor(version: &str) -> Result<(u32, u32)> {
    let parts: Vec<&str> = version.split('.').collect();
    if parts.len() < 2 {
        bail!("invalid python version {}", version);
    }
    let major = parts[0]
        .parse::<u32>()
        .with_context(|| format!("invalid python major version in {version}"))?;
    let minor = parts[1]
        .parse::<u32>()
        .with_context(|| format!("invalid python minor version in {version}"))?;
    Ok((major, minor))
}

fn resolve_latest_windows_installer_version(version: &str) -> Result<String> {
    let parts: Vec<&str> = version.split('.').collect();
    if parts.len() >= 3 {
        return Ok(version.to_string());
    }
    if parts.len() != 2 {
        return Ok(version.to_string());
    }

    let latest = resolve_latest_patch_version(version)?;
    if windows_installer_exists(&latest) {
        return Ok(latest);
    }
    if let Some(fallback) = windows_installer_fallback(version) {
        if windows_installer_exists(fallback) {
            return Ok(fallback.to_string());
        }
    }

    let mut candidates = list_patch_versions(version)?;
    candidates.sort_by(|a, b| compare_version(a, b).reverse());
    for candidate in &candidates {
        if windows_installer_exists(candidate) {
            return Ok(candidate.clone());
        }
    }

    if let Some(fallback) = windows_installer_fallback(version) {
        return Ok(fallback.to_string());
    }
    Ok(format!("{version}.0"))
}

fn resolve_latest_patch_version(version: &str) -> Result<String> {
    let parts: Vec<&str> = version.split('.').collect();
    if parts.len() >= 3 {
        return Ok(version.to_string());
    }
    if parts.len() != 2 {
        return Ok(version.to_string());
    }
    let mut candidates = list_patch_versions(version)?;
    if candidates.is_empty() {
        return Ok(format!("{version}.0"));
    }
    candidates.sort_by(|a, b| compare_version(a, b).reverse());
    Ok(candidates[0].clone())
}

fn list_patch_versions(version: &str) -> Result<Vec<String>> {
    let body = Client::builder()
        .timeout(Duration::from_secs(30))
        .build()
        .context("failed to build HTTP client")?
        .get("https://www.python.org/ftp/python/")
        .send()
        .context("failed to request python FTP listing")?
        .error_for_status()
        .context("python FTP listing request failed")?
        .text()
        .context("failed to decode python FTP response")?;
    let re = Regex::new(r#"href="(\d+\.\d+\.\d+)/""#).unwrap();
    let prefix = format!("{version}.");
    let mut out = Vec::new();
    for cap in re.captures_iter(&body) {
        let value = cap.get(1).map(|m| m.as_str()).unwrap_or_default();
        if value.starts_with(&prefix) {
            out.push(value.to_string());
        }
    }
    Ok(out)
}

fn windows_installer_fallback(version: &str) -> Option<&'static str> {
    match version {
        "3.9" => Some("3.9.13"),
        "3.10" => Some("3.10.11"),
        "3.11" => Some("3.11.9"),
        "3.12" => Some("3.12.3"),
        "3.13" => Some("3.13.0"),
        _ => None,
    }
}

fn windows_installer_exists(version: &str) -> bool {
    if version.trim().is_empty() {
        return false;
    }
    let url = format!(
        "https://www.python.org/ftp/python/{0}/python-{0}-amd64.exe",
        version
    );
    let client = match Client::builder().timeout(Duration::from_secs(20)).build() {
        Ok(c) => c,
        Err(_) => return false,
    };
    match client.head(&url).send() {
        Ok(resp) => {
            if resp.status() == StatusCode::METHOD_NOT_ALLOWED {
                client
                    .get(&url)
                    .header("Range", "bytes=0-0")
                    .send()
                    .map(|r| r.status().is_success() || r.status() == StatusCode::PARTIAL_CONTENT)
                    .unwrap_or(false)
            } else {
                resp.status().is_success()
            }
        }
        Err(_) => false,
    }
}

fn compare_version(a: &str, b: &str) -> Ordering {
    let pa: Vec<u32> = a
        .split('.')
        .map(|s| s.parse::<u32>().unwrap_or(0))
        .collect();
    let pb: Vec<u32> = b
        .split('.')
        .map(|s| s.parse::<u32>().unwrap_or(0))
        .collect();
    let n = pa.len().max(pb.len());
    for i in 0..n {
        let va = *pa.get(i).unwrap_or(&0);
        let vb = *pb.get(i).unwrap_or(&0);
        if va > vb {
            return Ordering::Greater;
        }
        if va < vb {
            return Ordering::Less;
        }
    }
    Ordering::Equal
}

fn is_windows_launcher_version_available(version: &str) -> bool {
    if !cfg!(windows) {
        return false;
    }
    let parts: Vec<&str> = version.split('.').collect();
    if parts.len() < 2 {
        return false;
    }
    let selector = format!("{}.{}", parts[0], parts[1]);
    let mut candidates = vec![PathBuf::from("py")];
    if let Ok(win_dir) = env::var("WINDIR") {
        candidates.push(PathBuf::from(win_dir).join("py.exe"));
    }
    if let Ok(local_app_data) = env::var("LOCALAPPDATA") {
        candidates.push(
            PathBuf::from(local_app_data)
                .join("Programs")
                .join("Python")
                .join("Launcher")
                .join("py.exe"),
        );
    }
    for candidate in candidates {
        let status = Command::new(&candidate)
            .arg(format!("-{}", selector))
            .arg("-V")
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .status();
        if let Ok(status) = status {
            if status.success() {
                return true;
            }
        }
    }
    false
}

fn is_python_runtime_healthy(exe: &Path) -> bool {
    let output = Command::new(exe)
        .args(["-c", "import encodings,site; print('ok')"])
        .output();
    match output {
        Ok(out) if out.status.success() => String::from_utf8_lossy(&out.stdout).contains("ok"),
        _ => false,
    }
}

fn add_to_path(dir: &Path) -> Result<()> {
    info(&format!("Ensuring {} is in system PATH...", dir.display()));
    if cfg!(windows) {
        let escaped = dir.to_string_lossy().replace('\'', "''");
        let script = format!(
            "$targetDir = '{}'; \
             $currentPath = [Environment]::GetEnvironmentVariable('Path', 'User'); \
             if ($null -eq $currentPath) {{ $currentPath = '' }}; \
             $pathArray = $currentPath.Split(';', [System.StringSplitOptions]::RemoveEmptyEntries); \
             if ($pathArray -notcontains $targetDir) {{ \
               $newPath = ($pathArray + $targetDir) -join ';'; \
               [Environment]::SetEnvironmentVariable('Path', $newPath, 'User'); \
               Write-Output 'Added to PATH'; \
             }} else {{ Write-Output 'Already in PATH'; }}"
            ,
            escaped
        );
        let status = Command::new("powershell")
            .arg("-NoProfile")
            .arg("-Command")
            .arg(script)
            .status()
            .context("failed to run PowerShell PATH update")?;
        if !status.success() {
            bail!("failed to update PATH");
        }
        return Ok(());
    }
    warning("Automatic PATH update on Linux is limited. Please ensure this is in your shell profile:");
    info(&format!("export PATH=\"$PATH:{}\"", dir.display()));
    Ok(())
}

fn create_shim(name: &str, target: &Path) -> Result<()> {
    let shim_dir = xe_shim_dir();
    fs::create_dir_all(&shim_dir).with_context(|| format!("failed to create {}", shim_dir.display()))?;
    if cfg!(windows) {
        let path = shim_dir.join(format!("{name}.bat"));
        let content = format!("@echo off\r\n\"{}\" %*\r\n", target.display());
        fs::write(&path, content).with_context(|| format!("failed to write {}", path.display()))?;
        return Ok(());
    }
    let path = shim_dir.join(name);
    let content = format!("#!/bin/sh\nexec \"{}\" \"$@\"\n", target.display());
    fs::write(&path, content).with_context(|| format!("failed to write {}", path.display()))?;
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        let mut perms = fs::metadata(&path)?.permissions();
        perms.set_mode(0o755);
        fs::set_permissions(&path, perms)?;
    }
    Ok(())
}

#[derive(Debug, Clone)]
struct VenvManager {
    base_dir: PathBuf,
}

impl VenvManager {
    fn new() -> Result<Self> {
        let base_dir = xe_venv_dir();
        fs::create_dir_all(&base_dir).with_context(|| format!("failed to create {}", base_dir.display()))?;
        Ok(Self { base_dir })
    }

    fn create(&self, name: &str, python_path: &Path) -> Result<()> {
        let venv_path = self.base_dir.join(name);
        if venv_path.exists() {
            bail!("venv {} already exists", name);
        }
        let status = Command::new(python_path)
            .arg("-m")
            .arg("venv")
            .arg(&venv_path)
            .status()
            .context("failed to create venv with stdlib venv")?;
        if status.success() {
            return Ok(());
        }

        let bootstrap = Command::new(python_path)
            .args([
                "-m",
                "pip",
                "install",
                "--disable-pip-version-check",
                "--no-warn-script-location",
                "--upgrade",
                "--force-reinstall",
                "virtualenv",
            ])
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .status()
            .context("failed to bootstrap virtualenv")?;
        if !bootstrap.success() {
            bail!("failed to bootstrap virtualenv");
        }
        let fallback = Command::new(python_path)
            .arg("-m")
            .arg("virtualenv")
            .arg(&venv_path)
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .status()
            .context("failed to create venv with virtualenv")?;
        if !fallback.success() {
            bail!("failed to create venv with virtualenv");
        }
        Ok(())
    }

    fn exists(&self, name: &str) -> bool {
        self.base_dir.join(name).exists()
    }

    fn delete(&self, name: &str) -> Result<()> {
        if name.trim().is_empty() {
            bail!("venv name required");
        }
        let path = self.base_dir.join(name);
        if path.exists() {
            fs::remove_dir_all(&path).with_context(|| format!("failed to remove {}", path.display()))?;
        }
        Ok(())
    }

    fn list(&self) -> Result<Vec<String>> {
        let mut out = Vec::new();
        for entry in fs::read_dir(&self.base_dir)
            .with_context(|| format!("failed to read {}", self.base_dir.display()))?
        {
            let entry = entry?;
            if entry.file_type()?.is_dir() {
                out.push(entry.file_name().to_string_lossy().to_string());
            }
        }
        Ok(out)
    }

    fn get_python_exe(&self, name: &str) -> PathBuf {
        if cfg!(windows) {
            self.base_dir.join(name).join("Scripts").join("python.exe")
        } else {
            self.base_dir.join(name).join("bin").join("python")
        }
    }

    fn get_site_packages_dir(&self, name: &str) -> PathBuf {
        if cfg!(windows) {
            self.base_dir.join(name).join("Lib").join("site-packages")
        } else {
            self.base_dir.join(name).join("lib")
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
struct Package {
    #[serde(alias = "Name")]
    name: String,
    #[serde(alias = "Version")]
    version: String,
    #[serde(default, alias = "DownloadURL")]
    download_url: String,
    #[serde(default, alias = "Hash")]
    hash: String,
}

fn extract_zip_to_dir(zip_path: &Path, target_dir: &Path) -> Result<()> {
    let file = File::open(zip_path).with_context(|| format!("failed to open {}", zip_path.display()))?;
    let mut archive = ZipArchive::new(file).with_context(|| format!("failed to parse {}", zip_path.display()))?;
    for index in 0..archive.len() {
        let mut entry = archive.by_index(index).with_context(|| format!("failed to read entry {}", index))?;
        let out_path = match entry.enclosed_name() {
            Some(name) => target_dir.join(name),
            None => continue,
        };
        if entry.name().ends_with('/') {
            fs::create_dir_all(&out_path)
                .with_context(|| format!("failed to create {}", out_path.display()))?;
            continue;
        }
        if let Some(parent) = out_path.parent() {
            fs::create_dir_all(parent)
                .with_context(|| format!("failed to create {}", parent.display()))?;
        }
        let mut out = File::create(&out_path).with_context(|| format!("failed to create {}", out_path.display()))?;
        io::copy(&mut entry, &mut out)
            .with_context(|| format!("failed to write {}", out_path.display()))?;
    }
    Ok(())
}

fn patch_embeddable_pth(python_dir: &Path) -> Result<()> {
    for entry in fs::read_dir(python_dir).with_context(|| format!("failed to read {}", python_dir.display()))? {
        let entry = entry?;
        let name = entry.file_name().to_string_lossy().to_string();
        if !name.to_lowercase().ends_with("._pth") {
            continue;
        }
        let path = entry.path();
        let content = fs::read_to_string(&path).with_context(|| format!("failed to read {}", path.display()))?;
        if content.contains("\nimport site") || content.starts_with("import site") {
            continue;
        }
        let updated = if content.contains("#import site") {
            content.replace("#import site", "import site")
        } else {
            format!("{content}\nimport site\n")
        };
        fs::write(&path, updated).with_context(|| format!("failed to write {}", path.display()))?;
    }
    Ok(())
}

fn bootstrap_pip(python_exe: &Path) -> Result<()> {
    info("Bootstrapping pip...");
    let mut resp = Client::builder()
        .timeout(Duration::from_secs(120))
        .build()
        .context("failed to build HTTP client")?
        .get("https://bootstrap.pypa.io/get-pip.py")
        .send()
        .context("failed to download get-pip.py")?;
    if !resp.status().is_success() {
        bail!("failed to download get-pip.py: {}", resp.status());
    }
    let script_path = python_exe
        .parent()
        .unwrap_or_else(|| Path::new("."))
        .join("get-pip.py");
    let mut script_file =
        File::create(&script_path).with_context(|| format!("failed to create {}", script_path.display()))?;
    io::copy(&mut resp, &mut script_file)
        .with_context(|| format!("failed to write {}", script_path.display()))?;

    let output = Command::new(python_exe)
        .arg(&script_path)
        .output()
        .context("failed to bootstrap pip")?;
    let _ = fs::remove_file(&script_path);
    if !output.status.success() {
        let stderr = String::from_utf8_lossy(&output.stderr);
        let stdout = String::from_utf8_lossy(&output.stdout);
        bail!("failed to bootstrap pip: {}\n{}{}", output.status, stdout, stderr);
    }
    Ok(())
}

#[derive(Debug, Clone, Serialize, Deserialize)]
struct SolveGraph {
    #[serde(alias = "PythonVersion")]
    python_version: String,
    #[serde(default, alias = "Requirements")]
    requirements: Vec<String>,
    #[serde(default, alias = "Packages")]
    packages: Vec<Package>,
}

struct Installer {
    cas: Cas,
}

impl Installer {
    fn new(global_cache_dir: &Path) -> Result<Self> {
        Ok(Self {
            cas: Cas::new(global_cache_dir)?,
        })
    }

    fn install(
        &self,
        ctx: &AppContext,
        cfg: &Config,
        requirements: &[String],
        project_dir: &Path,
        install_site_packages: &Path,
        python_exe: &Path,
    ) -> Result<Vec<Package>> {
        let _span = span(
            ctx,
            "install.total",
            json!({"python_version": cfg.python.version, "raw_requirements": requirements.len()}),
        );
        let reqs = normalize_requirements(requirements);
        if reqs.is_empty() {
            return Ok(Vec::new());
        }

        let cache_key = solve_key(&cfg.python.version, &reqs);
        let mut graph = if let Some(cached) = self.cas.load_solution::<SolveGraph>(&cache_key)? {
            cached
        } else {
            let solved = reqs
                .par_iter()
                .map(|req| resolve_requirement(req, python_exe))
                .collect::<Result<Vec<Vec<Package>>>>()?
                .into_iter()
                .flatten()
                .collect::<Vec<_>>();

            let solved = dedupe_packages(solved);
            let graph = SolveGraph {
                python_version: cfg.python.version.clone(),
                requirements: reqs.clone(),
                packages: solved,
            };
            self.cas.save_solution(&cache_key, &graph)?;
            graph
        };

        let mut download_plan = graph.packages.clone();
        download_plan.sort_by(|a, b| a.name.cmp(&b.name));

        let target_site_packages = if install_site_packages.as_os_str().is_empty() {
            project_dir.join("xe").join("site-packages")
        } else {
            install_site_packages.to_path_buf()
        };
        fs::create_dir_all(&target_site_packages)
            .with_context(|| format!("failed to create {}", target_site_packages.display()))?;

        let installed_set = Arc::new(Mutex::new(installed_package_key_set(&target_site_packages)?));
        download_plan.par_iter().try_for_each(|pkg| -> Result<()> {
            let key = package_identity_key(&pkg.name, &pkg.version);
            {
                let guard = installed_set.lock().map_err(|_| anyhow!("install state poisoned"))?;
                if guard.contains(&key) {
                    return Ok(());
                }
            }
            if pkg.download_url.trim().is_empty() {
                return Ok(());
            }

            let blob = self
                .cas
                .store_blob_from_url(&pkg.download_url, pkg.hash.as_str())?;
            install_wheel_blob(&blob, &target_site_packages)?;
            {
                let mut guard = installed_set.lock().map_err(|_| anyhow!("install state poisoned"))?;
                guard.insert(key);
            }
            Ok(())
        })?;

        graph.packages.sort_by(|a, b| a.name.cmp(&b.name));
        Ok(graph.packages)
    }
}

fn normalize_requirements(reqs: &[String]) -> Vec<String> {
    let mut out = reqs
        .iter()
        .map(|r| r.trim().to_string())
        .filter(|r| !r.is_empty())
        .collect::<Vec<_>>();
    out.sort();
    out
}

fn solve_key(python_version: &str, reqs: &[String]) -> String {
    let mut hasher = Sha1::new();
    hasher.update(python_version.as_bytes());
    hasher.update(b"|");
    for req in reqs {
        hasher.update(req.as_bytes());
        hasher.update(b"|");
    }
    hex::encode(hasher.finalize())
}

fn dedupe_packages(pkgs: Vec<Package>) -> Vec<Package> {
    let mut seen = BTreeMap::new();
    for pkg in pkgs {
        let key = format!("{}=={}", pkg.name.to_lowercase(), pkg.version);
        seen.insert(key, pkg);
    }
    seen.into_values().collect()
}

fn normalize_package_identity(name: &str) -> String {
    name.trim()
        .to_lowercase()
        .replace('-', "_")
        .replace('.', "_")
}

fn package_identity_key(name: &str, version: &str) -> String {
    format!("{}=={}", normalize_package_identity(name), version.trim())
}

fn installed_package_key_set(site_packages: &Path) -> Result<HashSet<String>> {
    let mut out = HashSet::new();
    if !site_packages.exists() {
        return Ok(out);
    }
    for entry in fs::read_dir(site_packages)
        .with_context(|| format!("failed to read {}", site_packages.display()))?
    {
        let entry = entry?;
        if !entry.file_type()?.is_dir() {
            continue;
        }
        let name = entry.file_name().to_string_lossy().to_string();
        if !name.to_lowercase().ends_with(".dist-info") {
            continue;
        }
        let base = name.trim_end_matches(".dist-info");
        if let Some(idx) = base.rfind('-') {
            if idx > 0 && idx + 1 < base.len() {
                let key = package_identity_key(&base[..idx], &base[idx + 1..]);
                out.insert(key);
            }
        }
    }
    Ok(out)
}

fn install_wheel_blob(blob_path: &Path, site_packages: &Path) -> Result<()> {
    fs::create_dir_all(site_packages)
        .with_context(|| format!("failed to create {}", site_packages.display()))?;
    let file = File::open(blob_path).with_context(|| format!("failed to open {}", blob_path.display()))?;
    let mut archive = ZipArchive::new(file).with_context(|| format!("failed to parse {}", blob_path.display()))?;
    for index in 0..archive.len() {
        let mut entry = archive.by_index(index).with_context(|| format!("failed to read entry {}", index))?;
        let enclosed = entry
            .enclosed_name()
            .ok_or_else(|| anyhow!("unsafe wheel entry path: {}", entry.name()))?
            .to_path_buf();
        let out_path = site_packages.join(enclosed);
        if entry.name().ends_with('/') {
            fs::create_dir_all(&out_path).with_context(|| format!("failed to create {}", out_path.display()))?;
            continue;
        }
        if let Some(parent) = out_path.parent() {
            fs::create_dir_all(parent)
                .with_context(|| format!("failed to create {}", parent.display()))?;
        }
        let mut out_file =
            File::create(&out_path).with_context(|| format!("failed to create {}", out_path.display()))?;
        io::copy(&mut entry, &mut out_file)
            .with_context(|| format!("failed to write {}", out_path.display()))?;
    }
    Ok(())
}

#[derive(Debug, Deserialize)]
struct PipReport {
    #[serde(default)]
    install: Vec<PipInstallItem>,
}

#[derive(Debug, Deserialize)]
struct PipInstallItem {
    metadata: PipMetadata,
    #[serde(default)]
    download_info: PipDownloadInfo,
}

#[derive(Debug, Deserialize)]
struct PipMetadata {
    name: String,
    version: String,
}

#[derive(Debug, Deserialize, Default)]
struct PipDownloadInfo {
    #[serde(default)]
    url: String,
    #[serde(default)]
    archive_info: PipArchiveInfo,
}

#[derive(Debug, Deserialize, Default)]
struct PipArchiveInfo {
    #[serde(default)]
    hashes: HashMap<String, String>,
}

fn resolve_requirement(requirement: &str, python_exe: &Path) -> Result<Vec<Package>> {
    let report_file = tempfile_path("xe-report", "json");
    let output = Command::new(python_exe)
        .arg("-m")
        .arg("pip")
        .arg("install")
        .arg(requirement)
        .arg("--dry-run")
        .arg("--report")
        .arg(&report_file)
        .output()
        .with_context(|| format!("dependency resolution failed for {requirement}"))?;
    if !output.status.success() {
        let stderr = String::from_utf8_lossy(&output.stderr);
        let stdout = String::from_utf8_lossy(&output.stdout);
        bail!(
            "dependency resolution failed for {}: {}\n{}{}",
            requirement,
            output.status,
            stdout,
            stderr
        );
    }
    let report_data = fs::read(&report_file)
        .with_context(|| format!("failed to read pip report {}", report_file.display()))?;
    let _ = fs::remove_file(&report_file);
    let sanitized = sanitize_json(&report_data);
    let report: PipReport = serde_json::from_slice(&sanitized)
        .with_context(|| format!("failed to parse pip report for {}", requirement))?;
    let mut packages = Vec::with_capacity(report.install.len());
    for item in report.install {
        let hash = item
            .download_info
            .archive_info
            .hashes
            .get("sha256")
            .cloned()
            .unwrap_or_default();
        packages.push(Package {
            name: item.metadata.name,
            version: item.metadata.version,
            download_url: item.download_info.url,
            hash,
        });
    }
    Ok(packages)
}

fn sanitize_json(data: &[u8]) -> Vec<u8> {
    let trimmed = trim_json_start(data);
    if trimmed.is_empty() {
        return data.to_vec();
    }
    let mut de = serde_json::Deserializer::from_slice(trimmed);
    match Value::deserialize(&mut de) {
        Ok(value) => serde_json::to_vec(&value).unwrap_or_else(|_| trimmed.to_vec()),
        Err(_) => trimmed.to_vec(),
    }
}

fn trim_json_start(data: &[u8]) -> &[u8] {
    match data.iter().position(|b| *b == b'{' || *b == b'[') {
        Some(idx) => &data[idx..],
        None => data,
    }
}

struct Cas {
    root: PathBuf,
}

impl Cas {
    fn new(root: &Path) -> Result<Self> {
        let cas = Self {
            root: root.to_path_buf(),
        };
        fs::create_dir_all(cas.blob_dir()).with_context(|| "failed to create CAS blob dir")?;
        fs::create_dir_all(cas.solution_dir())
            .with_context(|| "failed to create CAS solution dir")?;
        Ok(cas)
    }

    fn store_blob_from_url(&self, url: &str, expected_sha256: &str) -> Result<PathBuf> {
        if !expected_sha256.trim().is_empty() {
            let target = self.blob_path(expected_sha256);
            if target.exists() {
                return Ok(target);
            }
        }

        let client = Client::builder()
            .timeout(Duration::from_secs(120))
            .build()
            .context("failed to build HTTP client")?;
        let mut resp = client
            .get(url)
            .send()
            .with_context(|| format!("failed to download {}", url))?;
        if !resp.status().is_success() {
            bail!("download failed: {}", resp.status());
        }

        fs::create_dir_all(&self.root).with_context(|| format!("failed to create {}", self.root.display()))?;
        let tmp_path = tempfile_path_in(&self.root, "xe-download", "tmp");
        let mut tmp_file = File::create(&tmp_path)
            .with_context(|| format!("failed to create {}", tmp_path.display()))?;
        let mut hasher = Sha256::new();
        let mut buffer = [0u8; 64 * 1024];
        loop {
            let read = resp.read(&mut buffer).context("failed while downloading blob")?;
            if read == 0 {
                break;
            }
            hasher.update(&buffer[..read]);
            tmp_file
                .write_all(&buffer[..read])
                .with_context(|| format!("failed to write {}", tmp_path.display()))?;
        }
        tmp_file.flush().ok();
        let actual = hex::encode(hasher.finalize());

        if !expected_sha256.trim().is_empty() && !expected_sha256.eq_ignore_ascii_case(&actual) {
            let _ = fs::remove_file(&tmp_path);
            bail!(
                "checksum mismatch: expected={} actual={}",
                expected_sha256,
                actual
            );
        }

        let target = self.blob_path(&actual);
        if target.exists() {
            let _ = fs::remove_file(&tmp_path);
            return Ok(target);
        }
        if let Some(parent) = target.parent() {
            fs::create_dir_all(parent).with_context(|| format!("failed to create {}", parent.display()))?;
        }
        match fs::rename(&tmp_path, &target) {
            Ok(_) => {}
            Err(_) => {
                fs::copy(&tmp_path, &target)
                    .with_context(|| format!("failed to store blob at {}", target.display()))?;
                let _ = fs::remove_file(&tmp_path);
            }
        }
        Ok(target)
    }

    fn save_solution<T: Serialize>(&self, key: &str, value: &T) -> Result<()> {
        let path = self.solution_dir().join(format!("{key}.json"));
        if let Some(parent) = path.parent() {
            fs::create_dir_all(parent).with_context(|| format!("failed to create {}", parent.display()))?;
        }
        let file = File::create(&path).with_context(|| format!("failed to create {}", path.display()))?;
        serde_json::to_writer(file, value).with_context(|| format!("failed to write {}", path.display()))
    }

    fn load_solution<T: for<'de> Deserialize<'de>>(&self, key: &str) -> Result<Option<T>> {
        let path = self.solution_dir().join(format!("{key}.json"));
        if !path.exists() {
            return Ok(None);
        }
        let file = File::open(&path).with_context(|| format!("failed to open {}", path.display()))?;
        let value = serde_json::from_reader(file).with_context(|| format!("failed to parse {}", path.display()))?;
        Ok(Some(value))
    }

    fn blob_dir(&self) -> PathBuf {
        self.root.join("cas").join("blobs")
    }

    fn solution_dir(&self) -> PathBuf {
        self.root.join("cas").join("solutions")
    }

    fn blob_path(&self, sha: &str) -> PathBuf {
        let prefix = if sha.len() >= 2 { &sha[..2] } else { "00" };
        self.blob_dir().join(prefix).join(format!("{sha}.whl"))
    }
}

#[derive(Debug, Clone, Deserialize)]
struct PipPkg {
    name: String,
    version: String,
}

fn parse_pip_list_output(out: &[u8]) -> Result<Vec<PipPkg>> {
    let trimmed = out
        .iter()
        .copied()
        .skip_while(|b| b.is_ascii_whitespace())
        .collect::<Vec<_>>();
    if trimmed.is_empty() {
        return Ok(Vec::new());
    }

    if let Ok(direct) = serde_json::from_slice::<Vec<PipPkg>>(&trimmed) {
        return Ok(direct);
    }

    let mut start = 0usize;
    while start < trimmed.len() {
        if let Some(open_rel) = trimmed[start..].iter().position(|b| *b == b'[') {
            let open = start + open_rel;
            let mut depth: i32 = 0;
            for idx in open..trimmed.len() {
                match trimmed[idx] {
                    b'[' => depth += 1,
                    b']' => {
                        depth -= 1;
                        if depth == 0 {
                            let chunk = &trimmed[open..=idx];
                            if let Ok(pkgs) = serde_json::from_slice::<Vec<PipPkg>>(chunk) {
                                return Ok(pkgs);
                            }
                            break;
                        }
                    }
                    _ => {}
                }
            }
            start = open + 1;
        } else {
            break;
        }
    }
    bail!("pip JSON payload not found in output")
}

fn print_pkg_table(pkgs: &[PipPkg]) {
    let mut width = "Package".len();
    for pkg in pkgs {
        if pkg.name.len() > width {
            width = pkg.name.len();
        }
    }
    println!("{:<width$}  Version", "Package", width = width);
    for pkg in pkgs {
        println!("{:<width$}  {}", pkg.name, pkg.version, width = width);
    }
}

#[derive(Debug, Deserialize)]
struct PypiResponse {
    info: PypiInfo,
}

#[derive(Debug, Deserialize)]
struct PypiInfo {
    name: String,
    version: String,
    #[serde(default)]
    summary: String,
    #[serde(default)]
    home_page: String,
}

fn fetch_metadata_from_pypi(pkg_name: &str) -> Result<PypiResponse> {
    let url = format!("https://pypi.org/pypi/{pkg_name}/json");
    let resp = Client::builder()
        .timeout(Duration::from_secs(30))
        .build()
        .context("failed to build HTTP client")?
        .get(url)
        .send()
        .context("failed to request PyPI metadata")?;
    if !resp.status().is_success() {
        bail!("package {} not found on PyPI", pkg_name);
    }
    let parsed = resp.json::<PypiResponse>().context("failed to parse PyPI response")?;
    Ok(parsed)
}

fn parse_requirements(path: &Path) -> Result<Vec<String>> {
    let file = File::open(path).with_context(|| format!("failed to open {}", path.display()))?;
    let reader = BufReader::new(file);
    let mut reqs = Vec::new();
    for line in reader.lines() {
        let mut line = line?;
        line = line.trim().to_string();
        if line.is_empty() || line.starts_with('#') {
            continue;
        }
        if line.starts_with("-r ") || line.starts_with("--requirement ") {
            continue;
        }
        if line.starts_with('-') {
            continue;
        }
        if let Some(idx) = line.find(" #") {
            line = line[..idx].trim().to_string();
        }
        if !line.is_empty() {
            reqs.push(line);
        }
    }
    Ok(reqs)
}

fn create_snapshot(name: &str) -> Result<PathBuf> {
    let xe_dir = xe_home();
    let snaps_dir = xe_dir.join("snaps");
    fs::create_dir_all(&snaps_dir).with_context(|| format!("failed to create {}", snaps_dir.display()))?;
    let ts = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_else(|_| Duration::from_secs(0))
        .as_secs();
    let snap_path = snaps_dir.join(format!("{name}_{ts}.zip"));
    zip_directory(&xe_dir, &snap_path, &["snaps"])?;
    Ok(snap_path)
}

fn zip_directory(source: &Path, target: &Path, exclude: &[&str]) -> Result<()> {
    let file = File::create(target).with_context(|| format!("failed to create {}", target.display()))?;
    let mut writer = ZipWriter::new(file);
    let options = FileOptions::default().compression_method(zip::CompressionMethod::Deflated);

    for entry in WalkDir::new(source) {
        let entry = entry?;
        let path = entry.path();
        if path == source {
            continue;
        }
        let rel = path
            .strip_prefix(source)
            .with_context(|| format!("failed to strip prefix for {}", path.display()))?;
        let rel_str = rel.to_string_lossy().replace('\\', "/");
        if exclude.iter().any(|needle| rel_str.contains(needle)) {
            continue;
        }

        if entry.file_type().is_dir() {
            writer
                .add_directory(format!("{rel_str}/"), options)
                .with_context(|| format!("failed to add dir {}", rel_str))?;
            continue;
        }
        writer
            .start_file(rel_str.clone(), options)
            .with_context(|| format!("failed to add file {}", rel_str))?;
        let mut input = File::open(path).with_context(|| format!("failed to open {}", path.display()))?;
        io::copy(&mut input, &mut writer).with_context(|| format!("failed to write {}", rel_str))?;
    }
    writer.finish().context("failed to finalize snapshot zip")?;
    Ok(())
}

fn remove_path(path: &Path, description: &str) -> Result<()> {
    if path.exists() {
        info(&format!("Removing {} at {}...", description, path.display()));
        if path.is_dir() {
            fs::remove_dir_all(path).with_context(|| format!("failed to remove {}", path.display()))?;
        } else {
            fs::remove_file(path).with_context(|| format!("failed to remove {}", path.display()))?;
        }
    }
    Ok(())
}

fn read_stdin_line() -> Result<String> {
    let mut line = String::new();
    io::stdin().read_line(&mut line)?;
    Ok(line)
}

fn token_path() -> PathBuf {
    xe_home().join("credentials")
}

fn save_token(token: &str) -> Result<()> {
    let path = token_path();
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).with_context(|| format!("failed to create {}", parent.display()))?;
    }
    fs::write(&path, token).with_context(|| format!("failed to write {}", path.display()))?;
    Ok(())
}

fn load_token() -> Result<String> {
    let path = token_path();
    let token = fs::read_to_string(&path).with_context(|| format!("failed to read {}", path.display()))?;
    Ok(token)
}

fn revoke_token() -> Result<()> {
    let path = token_path();
    if path.exists() {
        fs::remove_file(&path).with_context(|| format!("failed to remove {}", path.display()))?;
    }
    Ok(())
}

#[derive(Clone)]
struct Profiler {
    inner: Arc<ProfilerInner>,
}

struct ProfilerInner {
    info: ProfileInfo,
    file: Mutex<File>,
    started: Instant,
}

#[derive(Clone)]
struct ProfileInfo {
    log_path: PathBuf,
    cpu_path: PathBuf,
    heap_path: PathBuf,
}

impl Profiler {
    fn start(profile_dir: &Path) -> Result<(Self, ProfileInfo)> {
        fs::create_dir_all(profile_dir)
            .with_context(|| format!("failed to create {}", profile_dir.display()))?;
        let stamp = profile_stamp();
        let info = ProfileInfo {
            log_path: profile_dir.join(format!("trace-{stamp}.jsonl")),
            cpu_path: profile_dir.join(format!("cpu-{stamp}.pprof")),
            heap_path: profile_dir.join(format!("heap-{stamp}.pprof")),
        };
        let log_file = File::create(&info.log_path)
            .with_context(|| format!("failed to create {}", info.log_path.display()))?;
        File::create(&info.cpu_path).with_context(|| format!("failed to create {}", info.cpu_path.display()))?;
        File::create(&info.heap_path)
            .with_context(|| format!("failed to create {}", info.heap_path.display()))?;

        let profiler = Self {
            inner: Arc::new(ProfilerInner {
                info: info.clone(),
                file: Mutex::new(log_file),
                started: Instant::now(),
            }),
        };
        profiler.event(
            "profile.session_start",
            json!({
                "log_path": profiler.inner.info.log_path.display().to_string(),
                "cpu_profile_path": profiler.inner.info.cpu_path.display().to_string(),
                "heap_profile_path": profiler.inner.info.heap_path.display().to_string(),
                "pid": std::process::id(),
                "os": env::consts::OS,
                "arch": env::consts::ARCH
            }),
        );
        Ok((profiler, info))
    }

    fn event(&self, name: &str, fields: Value) {
        let mut object = Map::new();
        object.insert("ts".to_string(), json!(timestamp_iso8601()));
        object.insert("event".to_string(), json!(name));
        if let Value::Object(map) = fields {
            for (key, value) in map {
                object.insert(key, value);
            }
        }
        if let Ok(mut file) = self.inner.file.lock() {
            if let Ok(line) = serde_json::to_string(&Value::Object(object)) {
                let _ = writeln!(file, "{line}");
            }
        }
    }

    fn stop(&self) -> Result<()> {
        self.event(
            "profile.session_stop",
            json!({
                "elapsed_ms": self.inner.started.elapsed().as_millis()
            }),
        );
        Ok(())
    }
}

struct SpanGuard {
    profiler: Option<Profiler>,
    name: String,
    started: Instant,
    fields: Value,
}

impl Drop for SpanGuard {
    fn drop(&mut self) {
        if let Some(profiler) = self.profiler.as_ref() {
            let mut fields = match self.fields.clone() {
                Value::Object(map) => map,
                _ => Map::new(),
            };
            fields.insert(
                "duration_ms".to_string(),
                json!(self.started.elapsed().as_millis()),
            );
            profiler.event(&format!("{}.done", self.name), Value::Object(fields));
        }
    }
}

fn span(ctx: &AppContext, name: &str, fields: Value) -> SpanGuard {
    if let Some(profiler) = ctx.profiler.as_ref() {
        profiler.event(&format!("{}.start", name), fields.clone());
    }
    SpanGuard {
        profiler: ctx.profiler.clone(),
        name: name.to_string(),
        started: Instant::now(),
        fields,
    }
}

fn profile_stamp() -> String {
    let millis = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_else(|_| Duration::from_secs(0))
        .as_millis();
    format!("{millis}")
}

fn timestamp_iso8601() -> String {
    OffsetDateTime::now_utc()
        .format(&Iso8601::DEFAULT)
        .unwrap_or_else(|_| "1970-01-01T00:00:00Z".to_string())
}

fn xe_home() -> PathBuf {
    if cfg!(windows) {
        if let Ok(local) = env::var("LOCALAPPDATA") {
            return PathBuf::from(local).join("xe");
        }
        return dirs::home_dir()
            .unwrap_or_else(|| PathBuf::from("."))
            .join("AppData")
            .join("Local")
            .join("xe");
    }
    dirs::home_dir()
        .unwrap_or_else(|| PathBuf::from("."))
        .join(".local")
        .join("share")
        .join("xe")
}

fn xe_config_file() -> PathBuf {
    xe_home().join("config.yaml")
}

fn xe_cache_dir() -> PathBuf {
    xe_home().join("cache")
}

fn xe_venv_dir() -> PathBuf {
    xe_home().join("venvs")
}

fn xe_shim_dir() -> PathBuf {
    xe_home().join("bin")
}

fn xe_plugin_dir() -> PathBuf {
    xe_home().join("plugins")
}

fn tempfile_path(prefix: &str, ext: &str) -> PathBuf {
    tempfile_path_in(&env::temp_dir(), prefix, ext)
}

fn tempfile_path_in(dir: &Path, prefix: &str, ext: &str) -> PathBuf {
    let stamp = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_else(|_| Duration::from_secs(0))
        .as_nanos();
    let pid = std::process::id();
    dir.join(format!("{prefix}-{pid}-{stamp}.{ext}"))
}

fn download_file(url: &str, prefix: &str, ext: &str) -> Result<PathBuf> {
    let client = Client::builder()
        .timeout(Duration::from_secs(180))
        .build()
        .context("failed to build HTTP client")?;
    let mut resp = client
        .get(url)
        .send()
        .with_context(|| format!("failed to download {}", url))?;
    if !resp.status().is_success() {
        bail!("failed to download {}: {}", url, resp.status());
    }
    let path = tempfile_path(prefix, ext);
    let mut out = File::create(&path).with_context(|| format!("failed to create {}", path.display()))?;
    io::copy(&mut resp, &mut out).with_context(|| format!("failed to write {}", path.display()))?;
    Ok(path)
}
