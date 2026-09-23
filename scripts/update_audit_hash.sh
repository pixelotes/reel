#!/bin/bash
# Calculate hash of the audit log
HASH=$(sha256sum var/log/audit_completion.log | awk '{print $1}')
echo "$HASH" > last_verification_hash
echo "Audit hash updated to $HASH"
