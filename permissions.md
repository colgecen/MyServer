# Permissions & Access Levels

## Level Definitions

| Level | Name            | Scope              | Capabilities                         | Confirmation Required |
|-------|-----------------|---------------------|--------------------------------------|-----------------------|
| 0     | READ_ONLY       | Workspace only     | Read files, list dirs, grep/search | No                    |
| 1     | WORKSPACE_WRITE | Workspace only     | Write/modify/delete workspace files | Yes (GUI Dialog)     |
| 2     | FULL_SYSTEM_EXEC| Entire system      | Arbitrary shell commands, root ops  | Yes (GUI Dialog + Audit Log) |

---

## Permission Matrix

```
COMMAND                    | Level | Auto-Approve | User Confirm | Notes
-------------------------------------------------------------------------------------------
ls, cat, find              | 0     | Yes          | No            | Read-only filesystem ops
grep, sed (read-only)      | 0     | Yes          | No            | Safe text inspection
mv, cp, touch              | 1     | No           | Yes           | File modifications within workspace
rm (workspace files)       | 1     | No           | Yes           | Deletions require explicit approval
rm -rf (non-root)          | 1     | No           | Yes           | Recursive workspace cleanup
npm install                | 1     | No           | Yes           | Package installation in workspace
systemctl restart          | 2     | No           | Yes + Audit   | Service management
sudo / su                  | 2     | No           | Yes + Audit   | Privilege escalation
shutdown / reboot          | 2     | No           | Yes + Audit   | System-wide operations
mkfs / format              | 2     | No           | Yes + Audit   | Destructive disk operations
```

---

## Level 1: WORKSPACE_WRITE

### Allowed Paths
```
Workspace paths selected via Workspace Selector:
/home/user/project/
/home/user/documents/notes/
```

### Restrictions
- Cannot exceed workspace root
- Cannot modify system files outside workspace
- File operations limited to user-owned files

### Examples
```
# Allowed
touch /home/user/project/newfile.txt
node build.js
rm /home/user/project/old.log

# Blocked (outside workspace)
touch /etc/motd
rm /home/other_user/config.txt
```

---

## Level 2: FULL_SYSTEM_EXEC

### Confirmation Requirements
- Mandatory user confirmation via modal dialog
- Command preview with full details
- 10-second timeout (auto-cancel if ignored)
- Audit log entry created before + after execution

### Examples
```
# Requires Level 2 approval
systemctl restart nginx
apt update && apt upgrade -y
dd if=/dev/zero of=/dev/sdX bs=1M count=10
```

---

## Permission Escalation Prevention

1. **Path Validation**  
   All file operations resolved against workspace root; paths outside rejected.

2. **Sudo Detection**  
   Commands prefixed with `sudo`, `su`, or containing privilege escalation triggers require Level 2 approval.

3. **Root Path Blocking**  
   Any path resolving to `/`, `/etc`, `/usr`, `/bin` requires Level 2 approval.

4. **Symlink Resolution**  
   Dangerous symlinks pointing outside workspace are blocked.

5. **Process Injection Prevention**  
   Shell metacharacters (`&&`, `||`, `;`) parsed safely; chained commands classified by highest-risk component.