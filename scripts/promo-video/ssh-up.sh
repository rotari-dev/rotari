#!/usr/bin/env bash
# Start an SSH server for a second host name inside the capture namespace.
#
#   ssh-up.sh SHIM HOST ADDR STATE_DIR
#
# Runs as the namespace's root (mapped to one host user), after /home/me is
# mounted. sshd gets its own UTS namespace, so commands run over `ssh HOST`
# see the host name HOST, and listens on ADDR (a loopback address) port 22;
# the file systems are shared, as on a cluster with a shared home. HOST
# resolves to ADDR, and ~/.ssh holds a key, known_hosts entry, and config so
# that `ssh HOST` logs in without prompts. SHIM is the setgroups shim that
# setup.sh builds for the Slurm image: sshd calls setgroups, which an
# unprivileged user namespace refuses. STATE_DIR keeps the keys,
# configuration, replacement /etc files, and sshd's log.
#
# In the namespace only uid 0 is mapped, so the passwd file mounted over
# /etc/passwd names uid 0 "me" (home /home/me) and gives sshd's privilege
# separation user uid 0 too; /run/sshd, its chroot, is replaced by an empty
# directory the namespace's root owns. nscd's socket is hidden so that name
# lookups read the mounted /etc/hosts instead of the host's cache.
set -euo pipefail
shim=$1
host=$2
addr=$3
state=$4
mkdir -p "$state"

grep -v '^root:\|^me:\|^sshd:' /etc/passwd >"$state/passwd.rest"
{
    echo "me:x:0:0:me:/home/me:/bin/bash"
    cat "$state/passwd.rest"
    echo "sshd:x:0:0:privilege separation:/run/sshd:/usr/sbin/nologin"
} >"$state/passwd"
printf '127.0.0.1 localhost workstation\n%s %s\n' "$addr" "$host" >"$state/hosts"
mount --bind "$state/passwd" /etc/passwd
mount --bind "$state/hosts" /etc/hosts
if [[ -d /run/nscd ]]; then
    mount -t tmpfs tmpfs /run/nscd
fi
if [[ ! -d /run/sshd ]]; then
    echo "ssh-up.sh: /run/sshd does not exist; install openssh-server" >&2
    exit 1
fi
mount -t tmpfs -o mode=755 tmpfs /run/sshd
ip addr add "$addr/8" dev lo 2>/dev/null || true

ssh-keygen -q -t ed25519 -N '' -f "$state/host_key" <<<y >/dev/null
mkdir -p /home/me/.ssh
chmod 700 /home/me/.ssh
ssh-keygen -q -t ed25519 -N '' -f /home/me/.ssh/id_ed25519 <<<y >/dev/null
cp /home/me/.ssh/id_ed25519.pub /home/me/.ssh/authorized_keys
printf '%s %s\n' "$host" "$(cat "$state/host_key.pub")" >/home/me/.ssh/known_hosts
printf 'Host %s\n    User me\n    BatchMode yes\n' "$host" >/home/me/.ssh/config
cat >"$state/sshd_config" <<EOF
Port 22
ListenAddress $addr
HostKey $state/host_key
PidFile $state/sshd.pid
UsePAM no
PasswordAuthentication no
KbdInteractiveAuthentication no
PubkeyAuthentication yes
AuthorizedKeysFile .ssh/authorized_keys
PermitRootLogin yes
EOF
unshare --uts sh -c 'hostname "$1"; shift; exec "$@"' _ "$host" \
    env LD_PRELOAD="$shim" /usr/sbin/sshd -f "$state/sshd_config" -E "$state/sshd.log"
for _ in $(seq 50); do
    (echo >"/dev/tcp/$addr/22") 2>/dev/null && break
    sleep 0.1
done
ssh "$host" true
