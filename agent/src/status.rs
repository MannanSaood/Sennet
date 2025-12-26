use anyhow::Result;
use std::process::Command;
use colored::*;

pub fn run() -> Result<()> {
    println!("{}", "Sennet Agent Status".bold().cyan());
    println!("{}", "===================".bold().cyan());

    // 1. Service Status
    let service_status = check_service_status();
    match service_status.as_str() {
        "active" => println!("Status:       {}", "Active (Running)".green().bold()),
        "inactive" => println!("Status:       {}", "Inactive".yellow()),
        "failed" => println!("Status:       {}", "Failed".red().bold()),
        _ => println!("Status:       {}", service_status),
    }

    if service_status != "active" {
        return Ok(());
    }

    // 2. Uptime & PID
    if let Ok((uptime, pid)) = get_service_details() {
        println!("PID:          {}", pid);
        println!("Uptime:       {}", uptime);
    }

    // 3. Interface (from config)
    if let Ok(interface) = get_interface_from_logs() {
        println!("Interface:    {}", interface);
    } else {
        println!("Interface:    {}", "Unknown".dimmed());
    }

    // 4. Backend Connection (from logs)
    if check_backend_connection() {
        println!("Backend:      {}", "Connected".green());
    } else {
        println!("Backend:      {}", "Disconnected / Error".red());
    }

    // 5. eBPF Mode
    println!("eBPF Mode:    {}", "TC (Traffic Control)".cyan());

    Ok(())
}

fn check_service_status() -> String {
    let output = Command::new("systemctl")
        .arg("is-active")
        .arg("sennet")
        .output();

    match output {
        Ok(out) => String::from_utf8_lossy(&out.stdout).trim().to_string(),
        Err(_) => "unknown".to_string(),
    }
}

fn get_service_details() -> Result<(String, String)> {
    let output = Command::new("systemctl")
        .arg("show")
        .arg("sennet")
        .arg("--property=ActiveEnterTimestamp,MainPID")
        .output()?;

    let out_str = String::from_utf8_lossy(&output.stdout);
    let mut pid = String::new();
    let mut uptime = String::new();

    for line in out_str.lines() {
        if line.starts_with("MainPID=") {
            pid = line.replace("MainPID=", "");
        } else if line.starts_with("ActiveEnterTimestamp=") {
            uptime = line.replace("ActiveEnterTimestamp=", "");
        }
    }

    Ok((uptime, pid))
}

fn get_interface_from_logs() -> Result<String> {
    // Grep logs for "Network interface: "
    let output = Command::new("bash")
        .arg("-c")
        .arg("journalctl -u sennet -n 50 | grep 'Network interface:' | tail -n 1 | awk '{print $NF}'")
        .output()?;
        
    Ok(String::from_utf8_lossy(&output.stdout).trim().to_string())
}

fn check_backend_connection() -> bool {
    // Check for recent heartbeat success
    let output = Command::new("bash")
        .arg("-c")
        .arg("journalctl -u sennet -n 20 --since '2 minutes ago' | grep -E 'Heartbeat successful|heartbeat'")
        .output();

    match output {
        Ok(out) => !out.stdout.is_empty(),
        Err(_) => false,
    }
}
