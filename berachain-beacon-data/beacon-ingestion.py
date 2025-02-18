import sqlite3
import mysql.connector
from mysql.connector import Error, pooling
import requests
from datetime import datetime
from tqdm import tqdm
import argparse
import json
from concurrent.futures import ThreadPoolExecutor, as_completed
import threading
from requests.adapters import HTTPAdapter
from requests.sessions import Session
from urllib3.util.retry import Retry
import sys
import os
from dataclasses import dataclass
from typing import Dict, List, Optional, Tuple, Union, Any
import time
import yaml

@dataclass
class Transaction:
    hash: str
    height: int
    index: int
    gas_wanted: int
    gas_used: int
    fee: str
    memo: str
    events: List[Dict[str, Any]]
    messages: List[Dict[str, Any]]
    raw_log: str

@dataclass
class Evidence:
    height: int
    type: str
    validator: str
    total_voting_power: int
    timestamp: str
    raw_data: Dict[str, Any]

@dataclass
class ValidatorSignature:
    height: int
    validator_address: str
    timestamp: str
    signature: str
    block_id_flag: int
    voting_power: int
    proposer_priority: int

def load_validator_mappings(mappings_file=None):
    """Load validator address to name mappings from YAML file"""
    validator_names = {}
    
    if not mappings_file:
        default_path = '/app/validators.yaml'
        config_path = '/app/config/validators.yaml'
        
        # Check for custom config first
        if os.path.exists(config_path):
            print(f"\nUsing custom validator mappings from: {config_path}")
            mappings_file = config_path
        else:
            print(f"\nUsing default validator mappings from: {default_path}")
            mappings_file = default_path
            
    if not os.path.exists(mappings_file):
        print(f"\nWARNING: Validator mappings file not found at: {mappings_file}")
        return validator_names
        
    try:
        print(f"\nLoading validator mappings from: {mappings_file}")
        with open(mappings_file, 'r') as f:
            # Load YAML content
            data = yaml.safe_load(f)
            
            # Check if data has the expected structure
            if not isinstance(data, dict) or 'validators' not in data:
                print(f"Warning: Expected YAML with 'validators' key, got: {data}")
                return validator_names
                
            mappings = data['validators']
            if not isinstance(mappings, list):
                print(f"Warning: Expected list under 'validators' key, got: {type(mappings)}")
                return validator_names
                
            # Process mappings with case preservation
            for mapping in mappings:
                if isinstance(mapping, str):
                    try:
                        address, name = mapping.split(':', 1)
                        # Store original address for lookups, but use uppercase for storage
                        original_address = address.strip()
                        uppercase_address = original_address.upper()
                        name = name.strip()
                        
                        if not original_address or not name:
                            print(f"Warning: Empty address or name in mapping: {mapping}")
                            continue
                            
                        validator_names[uppercase_address] = {
                            'name': name,
                            'original_address': original_address
                        }
                        print(f"Loaded mapping: {original_address} -> {name}")
                    except ValueError as e:
                        print(f"Warning: Invalid format in mapping: {mapping}")
                        continue
        
        print(f"\nSuccessfully loaded {len(validator_names)} validator mappings")
        if validator_names:
            print("\nAll loaded mappings:")
            for addr, info in sorted(validator_names.items()):
                print(f"  {info['original_address']} -> {info['name']}")
            print("\nAvailable addresses for matching:")
            print(", ".join(sorted(addr for addr in validator_names.keys())))
        
    except yaml.YAMLError as e:
        print(f"\nERROR parsing YAML file: {str(e)}")
        import traceback
        traceback.print_exc()
    except Exception as e:
        print(f"\nERROR loading validator mappings: {str(e)}")
        import traceback
        traceback.print_exc()
        
    return validator_names

class DatabaseConnectionPool:
    def __init__(self, db_type='sqlite', pool_size=5, **kwargs):
        self.db_type = db_type
        self.pool_size = pool_size
        self.kwargs = kwargs
        self.local = threading.local()
        self.pool = None
        
        if db_type == 'mysql':
            self._init_mysql_pool(pool_size, kwargs)
        else:
            self.database_path = kwargs.get('database', 'cosmos_data.db')

    def _init_mysql_pool(self, pool_size, kwargs):
        """Initialize MySQL connection pool with detailed error handling"""
        try:
            print("Attempting to create MySQL connection pool...")
            print(f"Connection parameters: host={kwargs.get('host')}, "
                  f"user={kwargs.get('user')}, database={kwargs.get('database')}, "
                  f"port={kwargs.get('port')}")
            
            # First try with mysql_native_password
            mysql_config = {
                'pool_name': "mypool",
                'pool_size': pool_size + 5,  # Add buffer for connection pool
                'pool_reset_session': True,  # Reset sessions when returning to pool
                'auth_plugin': 'mysql_native_password',
                'get_warnings': True,
                'raise_on_warnings': True,
                'connect_timeout': 60,  # Increase timeout
                'use_pure': True,  # Use pure Python implementation
                **kwargs
            }
            
            try:
                print("Trying mysql_native_password authentication...")
                self.pool = mysql.connector.pooling.MySQLConnectionPool(**mysql_config)
                print("Successfully created pool with mysql_native_password")
                return
            except mysql.connector.Error as err:
                print(f"Failed with mysql_native_password: {err}")
                if err.errno == 2059 or 'Authentication plugin' in str(err):  # Authentication plugin error
                    print("Attempting caching_sha2_password authentication...")
                    mysql_config['auth_plugin'] = 'caching_sha2_password'
                    try:
                        self.pool = mysql.connector.pooling.MySQLConnectionPool(**mysql_config)
                        print("Successfully created pool with caching_sha2_password")
                        return
                    except mysql.connector.Error as inner_err:
                        print(f"Failed with caching_sha2_password: {inner_err}")
                        
                        # Final attempt without specifying auth_plugin
                        print("Attempting connection without specifying auth_plugin...")
                        mysql_config.pop('auth_plugin', None)
                        try:
                            self.pool = mysql.connector.pooling.MySQLConnectionPool(**mysql_config)
                            print("Successfully created pool without auth_plugin specification")
                            return
                        except mysql.connector.Error as final_err:
                            print(f"Failed without auth_plugin: {final_err}")
                            raise final_err
                raise err

        except mysql.connector.Error as err:
            error_msg = (f"Fatal MySQL Error: {err}\n"
                        f"Error Code: {err.errno if hasattr(err, 'errno') else 'N/A'}\n"
                        f"Error Message: {err.msg if hasattr(err, 'msg') else str(err)}")
            print(error_msg, file=sys.stderr)
            print("\nTroubleshooting tips:")
            print("1. Verify MySQL server is running")
            print("2. Check credentials and permissions")
            print("3. Ensure database exists")
            print("4. Try updating mysql-connector-python:")
            print("   pip install mysql-connector-python --upgrade")
            print("5. Consider running these SQL commands:")
            print("   ALTER USER 'your_user'@'localhost' IDENTIFIED WITH mysql_native_password BY 'your_password';")
            print("   FLUSH PRIVILEGES;")
            raise

        except Exception as e:
            error_msg = f"Unexpected error creating connection pool: {str(e)}"
            print(error_msg, file=sys.stderr)
            raise

        finally:
            if not hasattr(self, 'pool') or self.pool is None:
                raise Exception("Failed to initialize MySQL connection pool")

    def get_connection(self):
        if self.db_type == 'mysql':
            if self.pool is None:
                raise Exception("MySQL connection pool was not properly initialized")
            
            try:
                conn = self.pool.get_connection()
                conn.autocommit = True
                return conn
            except mysql.connector.Error as err:
                print(f"Error getting MySQL connection: {err}", file=sys.stderr)
                raise
        else:
            # Create a new connection for each thread if it doesn't exist
            if not hasattr(self.local, 'connection'):
                self.local.connection = sqlite3.connect(self.database_path)
                self.local.connection.execute('PRAGMA journal_mode=WAL')
                self.local.cursor = self.local.connection.cursor()
            return self.local.connection, self.local.cursor

    def test_connection(self):
        """Test the database connection and print detailed information"""
        if self.db_type == 'mysql':
            if self.pool is None:
                raise Exception("MySQL connection pool was not properly initialized")
            
            try:
                conn = self.get_connection()
                cursor = conn.cursor()
                
                # Test basic query
                cursor.execute("SELECT VERSION()")
                version = cursor.fetchone()
                print(f"Successfully connected to MySQL version: {version[0]}")
                
                # Test database access
                cursor.execute("SHOW DATABASES")
                databases = cursor.fetchall()
                print(f"Available databases: {[db[0] for db in databases]}")
                
                # Test if our target database exists
                cursor.execute(f"USE {self.kwargs['database']}")
                print(f"Successfully connected to database: {self.kwargs['database']}")
                
                cursor.close()
                self.return_connection(conn)
                return True
            except mysql.connector.Error as err:
                print(f"MySQL connection test failed: {err}", file=sys.stderr)
                raise
        return True

    def return_connection(self, conn):
        if self.db_type == 'mysql':
            try:
                conn.close()
            except mysql.connector.Error as err:
                print(f"Error returning connection to pool: {err}", file=sys.stderr)
        # For SQLite, we keep the connection open for the thread

    def close_all(self):
        if self.db_type == 'sqlite':
            if hasattr(self.local, 'connection'):
                self.local.cursor.close()
                self.local.connection.close()

class RequestsSession:
    def __init__(self, retries=3, backoff_factor=0.3):
        self.session = Session()
        retry_strategy = Retry(
            total=retries,
            backoff_factor=backoff_factor,
            status_forcelist=[429, 500, 502, 503, 504]
        )
        adapter = HTTPAdapter(max_retries=retry_strategy, pool_maxsize=100)
        self.session.mount("http://", adapter)
        self.session.mount("https://", adapter)

class CosmosDataIngestion:
    def __init__(self, rpc_url, db_pool, batch_size=1000, max_workers=10, validator_mappings_file=None):
        self.cosmos_rpc_url = rpc_url
        self.db_pool = db_pool
        self.batch_size = batch_size
        self.max_workers = max_workers
        self.request_session = RequestsSession()
        self.validator_names = load_validator_mappings(validator_mappings_file)
        self.setup_tables()

        print("\nInitializing validator mappings...")
        self.validator_names = load_validator_mappings(validator_mappings_file)
        if not self.validator_names:
            print("WARNING: No validator mappings were loaded - names will be empty!")
        
        self.setup_tables()
        
    def _get_validator_name(self, address):
        """Get validator name from mapping with case-insensitive matching"""
        if not address:
            return ''
            
        # Convert address to uppercase for comparison
        address_upper = address.upper()
        
        # Get name with detailed logging
        name = self.validator_names.get(address_upper, '')
        
        if name:
            print(f"Address {address_upper} -> {name}")
        else:
            print(f"No mapping for {address_upper}")
            
        return name

    def get_last_processed_block(self) -> int:
        """Get the last processed block height from the ingestion_progress table"""
        if self.db_pool.db_type == 'mysql':
            conn = self.db_pool.get_connection()
            cursor = conn.cursor()
            
            try:
                cursor.execute('SELECT last_block_processed FROM ingestion_progress WHERE id = 1')
                result = cursor.fetchone()
                return result[0] if result else 0
            finally:
                cursor.close()
                self.db_pool.return_connection(conn)
                
        return 0

    def drop_tables(self):
        """Drop all existing tables"""
        if self.db_pool.db_type == 'mysql':
            conn = self.db_pool.get_connection()
            cursor = conn.cursor()
            
            try:
                # List all tables to drop
                tables = [
                    'cosmos_blocks',
                    'transactions',
                    'evidence',
                    'validators',
                    'signatures',
                    'validator_sets',
                    'ingestion_progress'
                ]
                
                # Disable foreign key checks temporarily
                cursor.execute("SET FOREIGN_KEY_CHECKS = 0")
                
                # Drop each table
                for table in tables:
                    print(f"Dropping table {table}...")
                    try:
                        cursor.execute(f"DROP TABLE IF EXISTS {table}")
                    except Exception as e:
                        print(f"Error dropping table {table}: {e}")
                
                # Re-enable foreign key checks
                cursor.execute("SET FOREIGN_KEY_CHECKS = 1")
                
                conn.commit()
                print("All tables dropped successfully")
                
            finally:
                cursor.close()
                self.db_pool.return_connection(conn)

    def setup_tables(self):
        """Setup database tables with permissions check"""
        if self.db_pool.db_type == 'mysql':
            conn = self.db_pool.get_connection()
            cursor = conn.cursor()
            
            try:
                # First, check permissions
                cursor.execute("SHOW GRANTS")
                grants = cursor.fetchall()
                print("\nDatabase permissions:")
                for grant in grants:
                    print(grant[0])

                # Get database name
                db_name = self.db_pool.kwargs['database']
                print(f"\nSetting up tables in database: {db_name}")

                # Get existing tables
                cursor.execute(f"SELECT TABLE_NAME FROM information_schema.tables WHERE table_schema = '{db_name}'")
                existing_tables = {row[0].lower() for row in cursor.fetchall()}
                print(f"\nExisting tables: {', '.join(existing_tables)}")

                # Table definitions
                tables = {
                    'cosmos_blocks': '''
                    CREATE TABLE cosmos_blocks (
                        height BIGINT PRIMARY KEY,
                        hash VARCHAR(64),
                        time DATETIME(3),
                        proposer_address VARCHAR(40),
                        proposer_name VARCHAR(100),
                        chain_id VARCHAR(64),
                        num_txs INT,
                        num_evidence INT,
                        total_gas_wanted BIGINT,
                        total_gas_used BIGINT,
                        total_fee DECIMAL(65,0),
                        last_commit_round INT,
                        last_block_id VARCHAR(64),
                        validators_hash VARCHAR(64),
                        next_validators_hash VARCHAR(64),
                        consensus_hash VARCHAR(64),
                        app_hash VARCHAR(64),
                        last_results_hash VARCHAR(64),
                        evidence_hash VARCHAR(64),
                        last_commit_hash VARCHAR(64),
                        data_hash VARCHAR(64),
                        valid_signatures INT,
                        total_signatures INT,
                        version VARCHAR(32),
                        parts_total INT,
                        parts_hash VARCHAR(64),
                        total_voting_power BIGINT,
                        proposer_priority BIGINT,
                        INDEX idx_time (time),
                        INDEX idx_proposer (proposer_address),
                        INDEX idx_proposer_name (proposer_name)
                    ) ENGINE=InnoDB
                    ''',
                    'transactions': '''
                    CREATE TABLE transactions (
                        hash VARCHAR(64) PRIMARY KEY,
                        height BIGINT,
                        tx_index INT,
                        gas_wanted BIGINT,
                        gas_used BIGINT,
                        fee DECIMAL(65,0),
                        memo TEXT,
                        events JSON,
                        messages JSON,
                        raw_log TEXT,
                        status VARCHAR(20),
                        codespace VARCHAR(100),
                        timestamp DATETIME(3),
                        INDEX idx_height (height),
                        INDEX idx_hash (hash)
                    ) ENGINE=InnoDB
                    ''',
                    'evidence': '''
                    CREATE TABLE evidence (
                        id BIGINT AUTO_INCREMENT PRIMARY KEY,
                        height BIGINT,
                        evidence_type VARCHAR(100),
                        validator_address VARCHAR(40),
                        total_voting_power BIGINT,
                        timestamp DATETIME(3),
                        raw_data JSON,
                        INDEX idx_height (height),
                        INDEX idx_validator (validator_address)
                    ) ENGINE=InnoDB
                    ''',
                    'validators': '''
                    CREATE TABLE validators (
                        address VARCHAR(40) PRIMARY KEY,
                        consensus_pubkey VARCHAR(100),
                        moniker VARCHAR(100),
                        website VARCHAR(200),
                        identity VARCHAR(100),
                        details TEXT,
                        commission_rate DECIMAL(5,2),
                        max_commission_rate DECIMAL(5,2),
                        max_commission_change_rate DECIMAL(5,2),
                        min_self_delegation BIGINT,
                        jailed BOOLEAN,
                        status VARCHAR(20),
                        tokens DECIMAL(65,0),
                        delegator_shares DECIMAL(65,0),
                        update_time DATETIME(3),
                        INDEX idx_status (status)
                    ) ENGINE=InnoDB
                    ''',
                    'signatures': '''
                    CREATE TABLE signatures (
                        id BIGINT AUTO_INCREMENT PRIMARY KEY,
                        height BIGINT,
                        validator_address VARCHAR(40),
                        timestamp DATETIME(3),
                        signature TEXT,
                        block_id_flag INT,
                        voting_power BIGINT,
                        proposer_priority BIGINT,
                        INDEX idx_height (height),
                        INDEX idx_validator (validator_address)
                    ) ENGINE=InnoDB
                    ''',
                    'validator_sets': '''
                    CREATE TABLE validator_sets (
                        height BIGINT,
                        validator_address VARCHAR(40),
                        voting_power BIGINT,
                        proposer_priority BIGINT,
                        PRIMARY KEY (height, validator_address),
                        INDEX idx_height (height),
                        INDEX idx_validator (validator_address)
                    ) ENGINE=InnoDB
                    ''',
                    'ingestion_progress': '''
                    CREATE TABLE ingestion_progress (
                        id INTEGER PRIMARY KEY,
                        last_block_processed BIGINT,
                        last_validator_update BIGINT
                    )
                    '''
                }
                alter_statements = {
                    'cosmos_blocks': 'ALTER TABLE cosmos_blocks MODIFY COLUMN time DATETIME(3);',
                    'transactions': 'ALTER TABLE transactions MODIFY COLUMN timestamp DATETIME(3);',
                    'evidence': 'ALTER TABLE evidence MODIFY COLUMN timestamp DATETIME(3);',
                    'signatures': 'ALTER TABLE signatures MODIFY COLUMN timestamp DATETIME(3);'
                }
                # Execute ALTER statements for existing tables
                for table, alter_sql in alter_statements.items():
                    try:
                        print(f"Updating timestamp precision for table {table}...")
                        cursor.execute(alter_sql)
                        print(f"Successfully updated {table}")
                    except Exception as e:
                        print(f"Error updating {table}: {e}")
                        # Continue with other tables even if one fails
                        continue
                # Create tables that don't exist
                for table_name, create_sql in tables.items():
                    if table_name.lower() not in existing_tables:
                        print(f"\nCreating table {table_name}...")
                        try:
                            cursor.execute(create_sql)
                            print(f"Successfully created table {table_name}")
                        except Exception as e:
                            print(f"Error creating table {table_name}: {e}")
                            raise
                    else:
                        print(f"\nTable {table_name} already exists, checking for updates...")
                        
                        # Add proposer_name column to cosmos_blocks table if it doesn't exist
                        if table_name == 'cosmos_blocks':
                            try:
                                print("Checking proposer_name column...")
                                cursor.execute('''
                                    SELECT COUNT(*) 
                                    FROM INFORMATION_SCHEMA.COLUMNS 
                                    WHERE TABLE_SCHEMA = DATABASE()
                                    AND TABLE_NAME = 'cosmos_blocks' 
                                    AND COLUMN_NAME = 'proposer_name'
                                ''')
                                if cursor.fetchone()[0] == 0:
                                    print("Adding proposer_name column...")
                                    cursor.execute('''
                                        ALTER TABLE cosmos_blocks 
                                        ADD COLUMN proposer_name VARCHAR(100) AFTER proposer_address
                                    ''')
                                    print("Successfully added proposer_name column")
                                else:
                                    print("proposer_name column already exists")

                                # Check and add index for proposer_name
                                cursor.execute('''
                                    SELECT COUNT(1) 
                                    FROM INFORMATION_SCHEMA.STATISTICS 
                                    WHERE TABLE_SCHEMA = DATABASE()
                                    AND TABLE_NAME = 'cosmos_blocks' 
                                    AND INDEX_NAME = 'idx_proposer_name'
                                ''')
                                if cursor.fetchone()[0] == 0:
                                    print("Adding index on proposer_name...")
                                    cursor.execute('''
                                        CREATE INDEX idx_proposer_name 
                                        ON cosmos_blocks(proposer_name)
                                    ''')
                                    print("Successfully added proposer_name index")
                                else:
                                    print("proposer_name index already exists")
                            except Exception as e:
                                print(f"Warning during table update: {e}")

                # Check if progress record exists
                cursor.execute("SELECT COUNT(*) FROM ingestion_progress WHERE id = 1")
                progress_exists = cursor.fetchone()[0] > 0

                if not progress_exists:
                    print("\nInitializing progress record...")
                    cursor.execute('''
                    INSERT INTO ingestion_progress (id, last_block_processed, last_validator_update) 
                    VALUES (1, 0, 0)
                    ''')
                    print("Progress record initialized")
                else:
                    print("\nProgress record already exists, skipping initialization")

                conn.commit()
                print("\nDatabase setup completed successfully")

            except Exception as e:
                print(f"\nError during table setup: {e}")
                conn.rollback()
                raise
            finally:
                cursor.close()
                self.db_pool.return_connection(conn)

    def _get_validator_name(self, address):
        """Get validator name from mapping with case-insensitive matching and original address preservation"""
        if not address:
            return ''
                
        # Convert address to uppercase for lookup
        address_upper = address.upper()
        
        # Get validator info
        validator_info = self.validator_names.get(address_upper)
        
        if validator_info:
            name = validator_info['name']
            original_address = validator_info['original_address']
            print(f"Found mapping for {original_address} -> {name}")
            return name
        else:
            print(f"No mapping found for {address}")
            return ''

    def _store_validator_set(self, cursor, height: int, validators: List[Dict]):
        """Store validator set data with names"""
        if self.db_pool.db_type == 'mysql':
            for validator in validators:
                address = validator.get('address')
                validator_name = self._get_validator_name(address)
                
                cursor.execute('''
                INSERT INTO validators (
                    address, validator_name, consensus_pubkey, moniker, website,
                    identity, details, commission_rate, max_commission_rate,
                    max_commission_change_rate, min_self_delegation, jailed,
                    status, tokens, delegator_shares, update_time
                ) VALUES (
                    %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, NOW()
                ) AS new_validator
                ON DUPLICATE KEY UPDATE
                    validator_name = new_validator.validator_name,
                    consensus_pubkey = new_validator.consensus_pubkey,
                    moniker = new_validator.moniker,
                    website = new_validator.website,
                    identity = new_validator.identity,
                    details = new_validator.details,
                    commission_rate = new_validator.commission_rate,
                    max_commission_rate = new_validator.max_commission_rate,
                    max_commission_change_rate = new_validator.max_commission_change_rate,
                    min_self_delegation = new_validator.min_self_delegation,
                    jailed = new_validator.jailed,
                    status = new_validator.status,
                    tokens = new_validator.tokens,
                    delegator_shares = new_validator.delegator_shares,
                    update_time = NOW()
                ''', (
                    address, validator_name,
                    validator.get('pub_key', {}).get('value'),
                    validator.get('description', {}).get('moniker'),
                    validator.get('description', {}).get('website'),
                    validator.get('description', {}).get('identity'),
                    validator.get('description', {}).get('details'),
                    validator.get('commission', {}).get('commission_rates', {}).get('rate'),
                    validator.get('commission', {}).get('commission_rates', {}).get('max_rate'),
                    validator.get('commission', {}).get('commission_rates', {}).get('max_change_rate'),
                    validator.get('min_self_delegation'),
                    validator.get('jailed'),
                    validator.get('status'),
                    validator.get('tokens'),
                    validator.get('delegator_shares')
                ))

    def fetch_block_data(self, height):
        try:
            # Fetch block data
            response = self.request_session.session.get(
                f"{self.cosmos_rpc_url}/block",
                params={"height": height},
                timeout=10
            )
            response.raise_for_status()
            result = response.json()
            
            if "result" not in result:
                return None
                
            block = result["result"].get("block")
            if not block:
                return None

            # Parse header information
            header = block.get("header", {})
            last_commit = block.get("last_commit", {})
            txs = block.get("data", {}).get("txs", [])
            evidence = block.get("evidence", {}).get("evidence", [])

            # Fetch validator set for this height to get voting power
            val_response = self.request_session.session.get(
                f"{self.cosmos_rpc_url}/validators",
                params={"height": height, "per_page": 100},  # Adjust per_page as needed
                timeout=10
            )
            val_response.raise_for_status()
            val_result = val_response.json()
            
            # Get proposer's voting power
            proposer_address = header.get("proposer_address", "")
            proposer_voting_power = 0
            proposer_priority = 0
            
            if "result" in val_result:
                validators = val_result["result"].get("validators", [])
                for validator in validators:
                    if validator.get("address") == proposer_address:
                        proposer_voting_power = int(validator.get("voting_power", 0))
                        proposer_priority = int(validator.get("proposer_priority", 0))
                        break

            # Get total gas and fees
            total_gas_wanted = 0
            total_gas_used = 0
            total_fee = "0"

            # Count valid signatures according to CometBFT block_id_flag spec:
            # BLOCK_ID_FLAG_UNKNOWN = 0  # error condition
            # BLOCK_ID_FLAG_ABSENT  = 1  # vote not received
            # BLOCK_ID_FLAG_COMMIT  = 2  # voted for the block that received majority
            # BLOCK_ID_FLAG_NIL     = 3  # voted for nil
            valid_signatures = sum(1 for sig in last_commit.get("signatures", [])
                                if isinstance(sig, dict) and 
                                sig.get("block_id_flag", 0) > 1 and  # Either COMMIT or NIL
                                sig.get("signature"))

            return {
                "height": int(header.get("height", 0)),
                "hash": result.get("block_id", {}).get("hash", ""),
                "time": header.get("time", ""),
                "proposer_address": proposer_address,
                "chain_id": header.get("chain_id", ""),
                "num_txs": len(txs),
                "num_evidence": len(evidence),
                "total_gas_wanted": total_gas_wanted,
                "total_gas_used": total_gas_used,
                "total_fee": total_fee,
                "last_commit_round": int(last_commit.get("round", 0)),
                "last_block_id": header.get("last_block_id", {}).get("hash", ""),
                "validators_hash": header.get("validators_hash", ""),
                "next_validators_hash": header.get("next_validators_hash", ""),
                "consensus_hash": header.get("consensus_hash", ""),
                "app_hash": header.get("app_hash", ""),
                "last_results_hash": header.get("last_results_hash", ""),
                "evidence_hash": header.get("evidence_hash", ""),
                "last_commit_hash": header.get("last_commit_hash", ""),
                "data_hash": header.get("data_hash", ""),
                "valid_signatures": valid_signatures,
                "total_signatures": len(last_commit.get("signatures", [])),
                "version": header.get("version", {}).get("block", ""),
                "parts_total": result.get("block_id", {}).get("parts", {}).get("total", 0),
                "parts_hash": result.get("block_id", {}).get("parts", {}).get("hash", ""),
                "total_voting_power": proposer_voting_power,  # Now contains proposer's voting power
                "proposer_priority": proposer_priority
            }
        except Exception as e:
            print(f"Error fetching block {height}: {str(e)}")
            return None

    def process_batch(self, start_height: int, end_height: int) -> int:
        if self.db_pool.db_type == 'mysql':
            conn = self.db_pool.get_connection()
            cursor = conn.cursor()
        else:
            conn, cursor = self.db_pool.get_connection()
        
        try:
            blocks_processed = 0
            for height in range(start_height, end_height + 1):
                block_data = self.fetch_block_data(height)
                if block_data:
                    try:
                        # Extract collections from block_data
                        transactions = block_data.pop('transactions', [])
                        evidence = block_data.pop('evidence', [])
                        signatures = block_data.pop('signatures', [])
                        validator_set = block_data.pop('validator_set', [])
                        
                        # Store block data
                        self._store_block(cursor, block_data)
                        
                        # Store related data
                        self._store_transactions(cursor, transactions)
                        self._store_evidence(cursor, evidence)
                        self._store_signatures(cursor, signatures)
                        self._store_validator_set(cursor, height, validator_set)
                        
                        # Update progress
                        cursor.execute('''
                        INSERT INTO ingestion_progress (id, last_block_processed, last_validator_update) 
                        VALUES (1, %s, %s) AS new_progress
                        ON DUPLICATE KEY UPDATE 
                            last_block_processed = new_progress.last_block_processed,
                            last_validator_update = new_progress.last_validator_update
                        ''', (height, height))
                        
                        blocks_processed += 1
                        conn.commit()  # Commit after each block
                        
                    except Exception as e:
                        print(f"Error processing block {height}: {str(e)}")
                        conn.rollback()
                        continue
            
            return blocks_processed
                
        except Exception as e:
            print(f"Error processing batch {start_height}-{end_height}: {str(e)}")
            conn.rollback()
            return 0
        finally:
            if self.db_pool.db_type == 'mysql':
                cursor.close()
                self.db_pool.return_connection(conn)

    def ingest_blocks(self, start_height, end_height):
        total_blocks = end_height - start_height + 1
        processed_blocks = 0
        
        with tqdm(total=total_blocks) as pbar:
            with ThreadPoolExecutor(max_workers=self.max_workers) as executor:
                futures = []
                
                for batch_start in range(start_height, end_height + 1, self.batch_size):
                    batch_end = min(batch_start + self.batch_size - 1, end_height)
                    future = executor.submit(self.process_batch, batch_start, batch_end)
                    futures.append(future)
                
                for future in as_completed(futures):
                    try:
                        blocks_processed = future.result()
                        processed_blocks += blocks_processed
                        pbar.update(blocks_processed)
                    except Exception as e:
                        print(f"Batch processing error: {str(e)}")
        
        return processed_blocks

    def _parse_timestamp(self, timestamp_str: str) -> str:
        """Convert Cosmos timestamp to MySQL compatible format with millisecond precision"""
        try:
            # Remove the 'Z' from the end if present
            timestamp_str = timestamp_str.rstrip('Z')
            
            # Parse the timestamp
            if '.' in timestamp_str:
                # Split into main part and fractional seconds
                main_part, fractional = timestamp_str.split('.')
                # Truncate fractional seconds to 6 digits (microseconds) before parsing
                fractional = fractional[:6]
                # Reconstruct timestamp string with truncated precision
                timestamp_str = f"{main_part}.{fractional}"
                dt = datetime.strptime(timestamp_str, '%Y-%m-%dT%H:%M:%S.%f')
            else:
                dt = datetime.strptime(timestamp_str, '%Y-%m-%dT%H:%M:%S')
                
            # Format for MySQL with millisecond precision
            return dt.strftime('%Y-%m-%d %H:%M:%S.%f')[:-3]  # Keep only milliseconds, not microseconds
        except Exception as e:
            print(f"Error parsing timestamp {timestamp_str}: {e}")
            # Return current time with millisecond precision if parsing fails
            return datetime.utcnow().strftime('%Y-%m-%d %H:%M:%S.%f')[:-3]

    def _store_block(self, cursor, block_data: Dict):
        """Store block data with millisecond precision timestamp"""
        if self.db_pool.db_type == 'mysql':
            # Convert timestamp to MySQL format with milliseconds
            mysql_timestamp = self._parse_timestamp(block_data['time'])
            
            # Get validator name, defaulting to empty string if not found
            proposer_address = block_data.get('proposer_address', '')
            proposer_name = self._get_validator_name(proposer_address)
            
            # Debug logging
            print(f"Storing block {block_data['height']}: proposer {proposer_address} -> {proposer_name}")
            
            cursor.execute('''
            INSERT INTO cosmos_blocks (
                height, hash, time, proposer_address, proposer_name, chain_id, 
                num_txs, num_evidence, total_gas_wanted, total_gas_used, total_fee, 
                last_commit_round, last_block_id, validators_hash, next_validators_hash, 
                consensus_hash, app_hash, last_results_hash, evidence_hash, 
                last_commit_hash, data_hash, valid_signatures, total_signatures, 
                version, parts_total, parts_hash, total_voting_power, proposer_priority
            ) VALUES (
                %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s,
                %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s
            ) AS new_values
            ON DUPLICATE KEY UPDATE
                hash = new_values.hash,
                time = new_values.time,
                proposer_address = new_values.proposer_address,
                proposer_name = new_values.proposer_name,
                chain_id = new_values.chain_id,
                num_txs = new_values.num_txs,
                num_evidence = new_values.num_evidence,
                total_gas_wanted = new_values.total_gas_wanted,
                total_gas_used = new_values.total_gas_used,
                total_fee = new_values.total_fee,
                last_commit_round = new_values.last_commit_round,
                last_block_id = new_values.last_block_id,
                validators_hash = new_values.validators_hash,
                next_validators_hash = new_values.next_validators_hash,
                consensus_hash = new_values.consensus_hash,
                app_hash = new_values.app_hash,
                last_results_hash = new_values.last_results_hash,
                evidence_hash = new_values.evidence_hash,
                last_commit_hash = new_values.last_commit_hash,
                data_hash = new_values.data_hash,
                valid_signatures = new_values.valid_signatures,
                total_signatures = new_values.total_signatures,
                version = new_values.version,
                parts_total = new_values.parts_total,
                parts_hash = new_values.parts_hash,
                total_voting_power = new_values.total_voting_power,
                proposer_priority = new_values.proposer_priority
            ''', (
                block_data['height'], 
                block_data['hash'], 
                mysql_timestamp,
                proposer_address, 
                proposer_name, 
                block_data['chain_id'],
                block_data['num_txs'], 
                block_data['num_evidence'],
                block_data['total_gas_wanted'], 
                block_data['total_gas_used'],
                block_data['total_fee'], 
                block_data['last_commit_round'],
                block_data['last_block_id'], 
                block_data['validators_hash'],
                block_data['next_validators_hash'], 
                block_data['consensus_hash'],
                block_data['app_hash'], 
                block_data['last_results_hash'],
                block_data['evidence_hash'], 
                block_data['last_commit_hash'],
                block_data['data_hash'], 
                block_data['valid_signatures'],
                block_data['total_signatures'], 
                block_data['version'],
                block_data['parts_total'], 
                block_data['parts_hash'],
                block_data['total_voting_power'], 
                block_data['proposer_priority']
            ))

    def _store_validator_set(self, cursor, height: int, validators: List[Dict]):
        """Store validator set data in the validator_sets and validators tables"""
        if self.db_pool.db_type == 'mysql':
            for validator in validators:
                # First update the validator info
                cursor.execute('''
                INSERT INTO validators (
                    address, consensus_pubkey, moniker, website,
                    identity, details, commission_rate, max_commission_rate,
                    max_commission_change_rate, min_self_delegation, jailed,
                    status, tokens, delegator_shares, update_time
                ) VALUES (
                    %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, NOW()
                ) AS new_validator
                ON DUPLICATE KEY UPDATE
                    consensus_pubkey = new_validator.consensus_pubkey,
                    moniker = new_validator.moniker,
                    website = new_validator.website,
                    identity = new_validator.identity,
                    details = new_validator.details,
                    commission_rate = new_validator.commission_rate,
                    max_commission_rate = new_validator.max_commission_rate,
                    max_commission_change_rate = new_validator.max_commission_change_rate,
                    min_self_delegation = new_validator.min_self_delegation,
                    jailed = new_validator.jailed,
                    status = new_validator.status,
                    tokens = new_validator.tokens,
                    delegator_shares = new_validator.delegator_shares,
                    update_time = NOW()
                ''', (
                    validator.get('address'),
                    validator.get('pub_key', {}).get('value'),
                    validator.get('description', {}).get('moniker'),
                    validator.get('description', {}).get('website'),
                    validator.get('description', {}).get('identity'),
                    validator.get('description', {}).get('details'),
                    validator.get('commission', {}).get('commission_rates', {}).get('rate'),
                    validator.get('commission', {}).get('commission_rates', {}).get('max_rate'),
                    validator.get('commission', {}).get('commission_rates', {}).get('max_change_rate'),
                    validator.get('min_self_delegation'),
                    validator.get('jailed'),
                    validator.get('status'),
                    validator.get('tokens'),
                    validator.get('delegator_shares')
                ))
                
                # Then store validator set info for this height
                cursor.execute('''
                INSERT INTO validator_sets (
                    height, validator_address, voting_power, proposer_priority
                ) VALUES (
                    %s, %s, %s, %s
                ) AS new_set
                ON DUPLICATE KEY UPDATE
                    voting_power = new_set.voting_power,
                    proposer_priority = new_set.proposer_priority
                ''', (
                    height,
                    validator.get('address'),
                    validator.get('voting_power'),
                    validator.get('proposer_priority')
                ))

    def _store_signatures(self, cursor, signatures: List[ValidatorSignature]):
        """Store signature data in the signatures table"""
        if self.db_pool.db_type == 'mysql':
            for sig in signatures:
                # Convert timestamp to MySQL format
                mysql_timestamp = self._parse_timestamp(sig.timestamp)
                
                cursor.execute('''
                INSERT INTO signatures (
                    height, validator_address, timestamp, signature,
                    block_id_flag, voting_power, proposer_priority
                ) VALUES (
                    %s, %s, %s, %s, %s, %s, %s
                )
                ''', (
                    sig.height, sig.validator_address, mysql_timestamp,
                    sig.signature, sig.block_id_flag, sig.voting_power,
                    sig.proposer_priority
                ))

    def _store_evidence(self, cursor, evidence_list: List[Evidence]):
        """Store evidence data in the evidence table"""
        if self.db_pool.db_type == 'mysql':
            for evidence in evidence_list:
                # Convert timestamp to MySQL format
                mysql_timestamp = self._parse_timestamp(evidence.timestamp)
                
                cursor.execute('''
                INSERT INTO evidence (
                    height, evidence_type, validator_address,
                    total_voting_power, timestamp, raw_data
                ) VALUES (
                    %s, %s, %s, %s, %s, %s
                )
                ''', (
                    evidence.height, evidence.type, evidence.validator,
                    evidence.total_voting_power, mysql_timestamp,
                    json.dumps(evidence.raw_data)
                ))

    def _store_transactions(self, cursor, transactions: List[Transaction]):
        """Store transaction data in the transactions table"""
        if self.db_pool.db_type == 'mysql':
            current_time = datetime.utcnow().strftime('%Y-%m-%d %H:%M:%S')
            for tx in transactions:
                cursor.execute('''
                INSERT INTO transactions (
                    hash, height, tx_index, gas_wanted, gas_used, fee,
                    memo, events, messages, raw_log, timestamp
                ) VALUES (
                    %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s
                ) AS new_tx
                ON DUPLICATE KEY UPDATE
                    height = new_tx.height,
                    tx_index = new_tx.tx_index,
                    gas_wanted = new_tx.gas_wanted,
                    gas_used = new_tx.gas_used,
                    fee = new_tx.fee,
                    memo = new_tx.memo,
                    events = new_tx.events,
                    messages = new_tx.messages,
                    raw_log = new_tx.raw_log,
                    timestamp = new_tx.timestamp
                ''', (
                    tx.hash, tx.height, tx.index, tx.gas_wanted, tx.gas_used,
                    tx.fee, tx.memo, json.dumps(tx.events), json.dumps(tx.messages),
                    tx.raw_log, current_time
                ))

def main():
    parser = argparse.ArgumentParser(description='Cosmos chain data ingestion')
    parser.add_argument('--validator-mappings', help='Path to validator mappings file')
    parser.add_argument('--db-type', choices=['sqlite', 'mysql'], default='mysql')
    parser.add_argument('--host', default='localhost')
    parser.add_argument('--user', default='root')
    parser.add_argument('--password', default='')
    parser.add_argument('--database', default='cosmos_data')
    parser.add_argument('--port', type=int, default=3306)
    parser.add_argument('--rpc-url', required=True, help='Cosmos RPC URL')
    parser.add_argument('--batch-size', type=int, default=100)
    parser.add_argument('--workers', type=int, default=5)
    parser.add_argument('--pool-size', type=int, default=10)
    parser.add_argument('--skip-setup', action='store_true', help='Skip database setup')
    
    args = parser.parse_args()
    
    db_params = {
        'host': args.host,
        'user': args.user,
        'password': args.password,
        'database': args.database,
        'port': args.port
    }
    
    print("\nInitializing database connection pool...")
    db_pool = DatabaseConnectionPool(args.db_type, args.pool_size, **db_params)
    
    ingestion = CosmosDataIngestion(
        args.rpc_url,
        db_pool,
        batch_size=args.batch_size,
        max_workers=args.workers,
        validator_mappings_file=args.validator_mappings
    )

    if not args.skip_setup:
        print("\nSetting up database tables...")
        ingestion.setup_tables()
    
    try:
        while True:
            # Get latest chain height
            latest_block_response = requests.get(
                f"{args.rpc_url}/abci_info",
                timeout=10
            )
            latest_block_response.raise_for_status()
            latest_height = int(latest_block_response.json()["result"]["response"]["last_block_height"])
            
            # Get our last processed height
            start_height = ingestion.get_last_processed_block() + 1
            
            if start_height > latest_height:
                print(f"\nCaught up at height {start_height-1}. Waiting for new blocks...")
                time.sleep(5)  # Wait 5 seconds before checking again
                continue
            
            # Process new blocks
            print(f"\nProcessing blocks {start_height} to {latest_height}")
            total_processed = ingestion.ingest_blocks(start_height, latest_height)
            print(f"Successfully processed {total_processed} blocks")
            
    except KeyboardInterrupt:
        print("\nShutting down gracefully...")
    except Exception as e:
        print(f"\nError occurred: {str(e)}")
        raise
    finally:
        if 'db_pool' in locals():
            db_pool.close_all()
if __name__ == "__main__":
    import signal
    
    def handle_sigterm(signum, frame):
        print("\nReceived termination signal - shutting down...")
        sys.exit(0)
    
    signal.signal(signal.SIGTERM, handle_sigterm)
    main()
