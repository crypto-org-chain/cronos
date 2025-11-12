#!/bin/bash

# Database Migration Swap Script
# This script replaces original databases with migrated ones and backs up the originals

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to validate backup suffix
validate_backup_suffix() {
    local suffix="$1"
    local sanitized
    
    # Remove any characters not in the safe set [A-Za-z0-9._-]
    sanitized=$(echo "$suffix" | tr -cd 'A-Za-z0-9._-')
    
    # Check if empty after sanitization
    if [[ -z "$sanitized" ]]; then
        echo -e "${RED}Error: BACKUP_SUFFIX is empty or contains only invalid characters${NC}" >&2
        echo -e "${RED}Allowed characters: A-Z, a-z, 0-9, period (.), underscore (_), hyphen (-)${NC}" >&2
        exit 1
    fi
    
    # Check if sanitized version differs from original (contains disallowed characters)
    if [[ "$suffix" != "$sanitized" ]]; then
        echo -e "${RED}Error: BACKUP_SUFFIX contains invalid characters: '$suffix'${NC}" >&2
        echo -e "${RED}Allowed characters: A-Z, a-z, 0-9, period (.), underscore (_), hyphen (-)${NC}" >&2
        echo -e "${RED}Sanitized version would be: '$sanitized'${NC}" >&2
        exit 1
    fi
    
    # Return success if validation passed
    return 0
}

# Default values
HOME_DIR="$HOME/.cronos"
DB_TYPE="app"
BACKUP_SUFFIX="backup-$(date +%Y%m%d-%H%M%S)"

# Validate default BACKUP_SUFFIX immediately after construction
validate_backup_suffix "$BACKUP_SUFFIX"

DRY_RUN=false

# Usage function
usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Swap migrated databases with originals and create backups.

OPTIONS:
    --home DIR           Node home directory (default: ~/.cronos)
    --db-type TYPE       Database type: app, cometbft, or all (default: app)
    --backup-suffix STR  Backup suffix (default: backup-YYYYMMDD-HHMMSS)
    --dry-run            Show what would be done without doing it
    -h, --help           Show this help message

EXAMPLES:
    # Swap application database
    $0 --home ~/.cronos --db-type app

    # Swap all CometBFT databases
    $0 --db-type cometbft

    # Swap all databases with custom backup name
    $0 --db-type all --backup-suffix before-rocksdb

    # Preview changes without executing
    $0 --db-type all --dry-run

EOF
    exit 1
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --home)
            HOME_DIR="$2"
            shift 2
            ;;
        --db-type)
            DB_TYPE="$2"
            shift 2
            ;;
        --backup-suffix)
            BACKUP_SUFFIX="$2"
            # Validate user-provided BACKUP_SUFFIX immediately
            validate_backup_suffix "$BACKUP_SUFFIX"
            shift 2
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        -h|--help)
            usage
            ;;
        *)
            echo -e "${RED}Error: Unknown option $1${NC}"
            usage
            ;;
    esac
done

# Validate db-type
if [[ "$DB_TYPE" != "app" && "$DB_TYPE" != "cometbft" && "$DB_TYPE" != "all" ]]; then
    echo -e "${RED}Error: Invalid db-type '$DB_TYPE'. Must be: app, cometbft, or all${NC}"
    exit 1
fi

# Validate home directory
if [[ ! -d "$HOME_DIR" ]]; then
    echo -e "${RED}Error: Home directory does not exist: $HOME_DIR${NC}"
    exit 1
fi

DATA_DIR="$HOME_DIR/data"
if [[ ! -d "$DATA_DIR" ]]; then
    echo -e "${RED}Error: Data directory does not exist: $DATA_DIR${NC}"
    exit 1
fi

# Determine which databases to swap
declare -a DB_NAMES
case "$DB_TYPE" in
    app)
        DB_NAMES=("application")
        ;;
    cometbft)
        DB_NAMES=("blockstore" "state" "tx_index" "evidence")
        ;;
    all)
        DB_NAMES=("application" "blockstore" "state" "tx_index" "evidence")
        ;;
esac

# Function to print colored output
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to get directory size
get_size() {
    if [[ -d "$1" ]]; then
        du -sh "$1" 2>/dev/null | awk '{print $1}'
    else
        echo "N/A"
    fi
}

# Check for migrated databases
print_info "Checking for migrated databases..."
FOUND_MIGRATED=false
declare -a AVAILABLE_DBS

for db_name in "${DB_NAMES[@]}"; do
    migrated_db="$DATA_DIR/${db_name}.migrate-temp.db"
    if [[ -d "$migrated_db" ]]; then
        FOUND_MIGRATED=true
        AVAILABLE_DBS+=("$db_name")
        print_info "  ✓ Found: ${db_name}.migrate-temp.db ($(get_size "$migrated_db"))"
    else
        print_warning "  ✗ Not found: ${db_name}.migrate-temp.db"
    fi
done

if [[ "$FOUND_MIGRATED" == false ]]; then
    print_error "No migrated databases found in $DATA_DIR"
    print_info "Run the migration first: cronosd migrate-db --db-type $DB_TYPE"
    exit 1
fi

echo ""
print_info "Database type: $DB_TYPE"
print_info "Home directory: $HOME_DIR"
print_info "Data directory: $DATA_DIR"
print_info "Backup suffix: $BACKUP_SUFFIX"
if [[ "$DRY_RUN" == true ]]; then
    print_warning "DRY RUN MODE - No changes will be made"
fi

# Create backup directory (skip in dry run to avoid side effects)
BACKUP_DIR="$DATA_DIR/backups-$BACKUP_SUFFIX"
if [[ "$DRY_RUN" == false ]]; then
    if ! mkdir -p "$BACKUP_DIR"; then
        print_error "Failed to create backup directory: $BACKUP_DIR"
        exit 1
    fi
fi

# Initialize counters
SUCCESS_COUNT=0
FAILED_COUNT=0

echo ""
echo "================================================================================"
echo "MIGRATION SWAP PLAN"
echo "================================================================================"

for db_name in "${AVAILABLE_DBS[@]}"; do
    original_db="$DATA_DIR/${db_name}.db"
    migrated_db="$DATA_DIR/${db_name}.migrate-temp.db"
    backup_db="$BACKUP_DIR/${db_name}.db"
    
    echo ""
    echo "Database: $db_name"
    echo "  Original: $original_db ($(get_size "$original_db"))"
    echo "  Migrated: $migrated_db ($(get_size "$migrated_db"))"
    echo "  Backup:   $backup_db"
done

echo ""
echo "================================================================================"

# Confirmation for non-dry-run
if [[ "$DRY_RUN" == false ]]; then
    echo ""
    print_warning "This will:"
    echo "  1. Move original databases to: $BACKUP_DIR"
    echo "  2. Replace with migrated databases"
    echo ""
    read -p "Continue? (yes/no): " -r
    echo ""
    if [[ ! $REPLY =~ ^[Yy][Ee][Ss]$ ]]; then
        print_info "Aborted by user"
        exit 0
    fi
fi

echo ""
print_info "Starting database swap..."
echo ""

# Perform the swap
for db_name in "${AVAILABLE_DBS[@]}"; do
    echo ""
    print_info "Processing: $db_name"
    
    original_db="$DATA_DIR/${db_name}.db"
    migrated_db="$DATA_DIR/${db_name}.migrate-temp.db"
    backup_db="$BACKUP_DIR/${db_name}.db"
    
    # Check if original exists
    if [[ ! -d "$original_db" ]]; then
        print_warning "  Original database not found, skipping backup: $original_db"
        ORIGINAL_EXISTS=false
    else
        ORIGINAL_EXISTS=true
    fi
    
    # Move original to backup if it exists
    if [[ "$ORIGINAL_EXISTS" == true ]]; then
        if [[ "$DRY_RUN" == false ]]; then
            print_info "  Moving original to backup..."
            mv "$original_db" "$backup_db"
            print_success "  ✓ Moved to backup: $original_db → $backup_db"
        else
            print_info "  [DRY RUN] Would move to backup: $original_db → $backup_db"
        fi
    fi
    
    # Move migrated to original location
    if [[ "$DRY_RUN" == false ]]; then
        print_info "  Installing migrated database..."
        mv "$migrated_db" "$original_db"
        print_success "  ✓ Moved: $migrated_db → $original_db"
        SUCCESS_COUNT=$((SUCCESS_COUNT + 1))
    else
        print_info "  [DRY RUN] Would move: $migrated_db → $original_db"
        SUCCESS_COUNT=$((SUCCESS_COUNT + 1))
    fi
done

echo ""
echo "================================================================================"
if [[ "$DRY_RUN" == false ]]; then
    echo -e "${GREEN}DATABASE SWAP COMPLETED SUCCESSFULLY${NC}"
else
    echo -e "${YELLOW}DRY RUN COMPLETED${NC}"
fi
echo "================================================================================"

if [[ "$DRY_RUN" == false ]]; then
echo ""
echo "Summary:"
echo "  Databases swapped: $SUCCESS_COUNT"
echo "  Backups location: $BACKUP_DIR"
echo ""
echo "Note: Original databases were moved (not copied) to backup location."
echo "      This is faster and saves disk space."
echo ""
echo "Next steps:"
echo "  1. Update your configuration files:"
    
    if [[ "$DB_TYPE" == "app" || "$DB_TYPE" == "all" ]]; then
        echo "     - Edit ~/.cronos/config/app.toml"
        echo "       Change: app-db-backend = \"rocksdb\"  # or your target backend"
    fi
    
    if [[ "$DB_TYPE" == "cometbft" || "$DB_TYPE" == "all" ]]; then
        echo "     - Edit ~/.cronos/config/config.toml"
        echo "       Change: db_backend = \"rocksdb\"  # or your target backend"
    fi
    
    echo ""
    echo "  2. Start your node:"
    echo "     systemctl start cronosd"
    echo "     # or"
    echo "     cronosd start --home $HOME_DIR"
    echo ""
    echo "  3. Monitor the logs to ensure everything works correctly"
    echo ""
    echo "  4. If everything works, you can remove the backups:"
    echo "     rm -rf $BACKUP_DIR"
    echo ""
else
    echo ""
    echo "This was a dry run. No changes were made."
    echo "Run without --dry-run to perform the actual swap."
    echo ""
fi

# List data directory
echo ""
print_info "Current data directory contents:"
ls -lh "$DATA_DIR" | grep -E "^d" | awk '{print "  " $9 " (" $5 ")"}'

echo ""
print_success "Script completed"

