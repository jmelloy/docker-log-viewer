# Settings Page Guide

The Settings page provides a user-friendly interface for managing servers and database URLs in Logseidon.

## Accessing the Settings Page

Navigate to `http://localhost:9000/settings.html` or click the **Settings** link in the navigation bar.

## Managing Servers

### Creating a Server

1. Click the **+ New Server** button
2. Fill in the form:
   - **Name** (required): A friendly name for your server
   - **URL** (required): The GraphQL/API endpoint URL
   - **Bearer Token** (optional): Authentication token
   - **Dev ID** (optional): Development/testing identifier
   - **Default Database** (optional): Select a database URL for EXPLAIN queries
3. Click **Save**

### Editing a Server

1. Click the **Edit** button next to the server you want to modify
2. Update the fields as needed
3. Click **Save**

### Deleting a Server

1. Click the **Delete** button next to the server you want to remove
2. Confirm the deletion when prompted

**Note:** Deleting a server will not affect sample queries that reference it, but they will need to specify a server when executed.

## Managing Database URLs

Database URLs are used for running EXPLAIN queries on SQL statements captured in logs.

### Creating a Database URL

1. Click the **+ New Database URL** button
2. Fill in the form:
   - **Name** (required): A friendly name for the database
   - **Database Type** (required): PostgreSQL or MySQL
   - **Connection String** (required): Full connection string (e.g., `postgresql://user:pass@host:5432/dbname`)
3. Click **Save**

### Editing a Database URL

1. Click the **Edit** button next to the database URL you want to modify
2. Update the fields as needed
3. Click **Save**

### Deleting a Database URL

1. Click the **Delete** button next to the database URL you want to remove
2. Confirm the deletion when prompted

**Warning:** If you delete a database URL that is set as a default for any servers, those servers will no longer have a default database for EXPLAIN queries.

## Using Servers and Databases Together

### Default Database Association

When creating or editing a server, you can select a **Default Database** from the dropdown. This database will be used automatically when running EXPLAIN queries for SQL statements captured from requests to that server.

### Benefits

1. **Centralized Configuration**: Manage all server and database configurations in one place
2. **Reusability**: Define servers and databases once, use them across multiple sample queries
3. **Security**: Store authentication tokens and connection strings securely in the database
4. **Flexibility**: Override server and database settings on a per-request basis via the API

## API Usage

The Settings page uses the following REST API endpoints. You can also use these directly:

### Server Endpoints

```bash
# List all servers
GET /api/servers

# Create a server
POST /api/servers
Content-Type: application/json
{
  "name": "Production API",
  "url": "https://api.example.com/graphql",
  "bearerToken": "token-here",
  "devId": "dev-123",
  "defaultDatabaseId": 1
}

# Get a specific server
GET /api/servers/:id

# Update a server
PUT /api/servers/:id
Content-Type: application/json
{ ... }

# Delete a server
DELETE /api/servers/:id
```

### Database URL Endpoints

```bash
# List all database URLs
GET /api/database-urls

# Create a database URL
POST /api/database-urls
Content-Type: application/json
{
  "name": "Production DB",
  "connectionString": "postgresql://user:pass@localhost:5432/prod",
  "databaseType": "postgresql"
}

# Get a specific database URL
GET /api/database-urls/:id

# Update a database URL
PUT /api/database-urls/:id
Content-Type: application/json
{ ... }

# Delete a database URL
DELETE /api/database-urls/:id
```

## Tips

- Use descriptive names for servers and databases to make them easy to identify
- Keep connection strings secure - they are stored in the SQLite database
- Test your database connections after creating them by running an EXPLAIN query
- Servers can be created without a default database - you can add one later
- The Settings page automatically refreshes the list after create/update/delete operations
