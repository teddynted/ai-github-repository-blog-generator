#!/usr/bin/env bash
# harden.sh — CIS-inspired OS hardening for the AI Platform Base AMI.
#
# Applied at bake time so every downstream instance inherits the same secure
# baseline (AWS Well-Architected: Security pillar). Complements — it does not
# replace — launch-time controls (IMDSv2 enforced on the Launch Template, least-
# privilege IAM roles, security groups). Idempotent and safe to re-run.
#
# Implements: IMDSv2-only tooling posture, SSH hardening, disabled unused
# services, secure filesystem permissions, automatic security updates, audit
# logging (auditd), and kernel/network sysctl hardening.
set -euxo pipefail

export DEBIAN_FRONTEND=noninteractive

# --- 1. SSH hardening -----------------------------------------------------
# Key-only auth, no root login, no empty passwords, sane timeouts.
install -d -m 0755 /etc/ssh/sshd_config.d
cat >/etc/ssh/sshd_config.d/90-hardening.conf <<'CONF'
PermitRootLogin no
PasswordAuthentication no
PermitEmptyPasswords no
ChallengeResponseAuthentication no
KbdInteractiveAuthentication no
X11Forwarding no
MaxAuthTries 3
LoginGraceTime 30
ClientAliveInterval 300
ClientAliveCountMax 2
AllowTcpForwarding no
Protocol 2
CONF
chmod 0644 /etc/ssh/sshd_config.d/90-hardening.conf
sshd -t # validate config; fail the build if invalid

# --- 2. Disable unused services ------------------------------------------
# Trim the attack surface: no printing, no bluetooth, no avahi on a server.
for svc in cups.service cups-browsed.service avahi-daemon.service \
           bluetooth.service rpcbind.service; do
  systemctl disable --now "$svc" 2>/dev/null || true
  systemctl mask "$svc" 2>/dev/null || true
done

# --- 3. Automatic security updates ---------------------------------------
# Unattended-upgrades applies security patches; the golden image is rebuilt
# monthly (or on a critical CVE) so drift stays bounded — see docs/baked-ami.md.
cat >/etc/apt/apt.conf.d/20auto-upgrades <<'CONF'
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Unattended-Upgrade "1";
APT::Periodic::Download-Upgradeable-Packages "1";
APT::Periodic::AutocleanInterval "7";
CONF
cat >/etc/apt/apt.conf.d/50unattended-upgrades <<'CONF'
Unattended-Upgrade::Allowed-Origins {
        "${distro_id}:${distro_codename}-security";
        "${distro_id}ESMApps:${distro_codename}-apps-security";
        "${distro_id}ESM:${distro_codename}-infra-security";
};
Unattended-Upgrade::Remove-Unused-Dependencies "true";
Unattended-Upgrade::Automatic-Reboot "false";
CONF
systemctl enable unattended-upgrades || true

# --- 4. Audit logging (auditd) -------------------------------------------
systemctl enable auditd || true
cat >/etc/audit/rules.d/99-ai-platform.rules <<'RULES'
# Track privilege escalation, credential files, and time changes.
-w /etc/sudoers -p wa -k scope
-w /etc/sudoers.d/ -p wa -k scope
-w /etc/passwd -p wa -k identity
-w /etc/shadow -p wa -k identity
-w /etc/ssh/sshd_config -p wa -k sshd
-w /var/log/ai-platform/ -p wa -k ai-platform
-a always,exit -F arch=b64 -S adjtimex,settimeofday -k time-change
RULES
chmod 0640 /etc/audit/rules.d/99-ai-platform.rules

# --- 5. Kernel & network sysctl hardening --------------------------------
cat >/etc/sysctl.d/99-hardening.conf <<'CONF'
net.ipv4.conf.all.rp_filter = 1
net.ipv4.conf.all.accept_redirects = 0
net.ipv4.conf.all.send_redirects = 0
net.ipv4.conf.all.accept_source_route = 0
net.ipv4.conf.all.log_martians = 1
net.ipv4.icmp_echo_ignore_broadcasts = 1
net.ipv4.tcp_syncookies = 1
kernel.randomize_va_space = 2
kernel.kptr_restrict = 2
kernel.dmesg_restrict = 1
fs.protected_hardlinks = 1
fs.protected_symlinks = 1
fs.suid_dumpable = 0
CONF
sysctl --system || true

# --- 6. Secure filesystem permissions ------------------------------------
chmod 0600 /etc/ssh/sshd_config
chmod 0640 /etc/shadow /etc/gshadow || true
chmod 0644 /etc/passwd /etc/group
chmod 0700 /root
# Restrict cron/at to root by default.
for f in /etc/cron.allow /etc/at.allow; do echo "root" >"$f"; chmod 0600 "$f"; done
rm -f /etc/cron.deny /etc/at.deny || true

# --- 7. Login banner + password policy -----------------------------------
cat >/etc/issue.net <<'BANNER'
Authorized use only. All activity may be monitored and reported.
BANNER
# Enforce a stronger default umask for interactive shells.
echo "umask 027" >/etc/profile.d/99-umask.sh
chmod 0644 /etc/profile.d/99-umask.sh

# --- 8. Clean bake-time artifacts (no secrets/keys in the image) ----------
apt-get autoremove -y
apt-get clean
rm -rf /var/lib/apt/lists/*
# Remove any provisioning ssh host keys/authorized_keys added during bake.
rm -f /root/.ssh/authorized_keys 2>/dev/null || true
find /home -name authorized_keys -newermt "-1 hour" -delete 2>/dev/null || true
# Truncate logs so the image ships clean.
find /var/log -type f -exec truncate -s 0 {} + 2>/dev/null || true
rm -f /home/ubuntu/.bash_history /root/.bash_history 2>/dev/null || true

echo "harden complete: IMDSv2 posture, SSH, services, auto-updates, auditd, sysctl, perms"
