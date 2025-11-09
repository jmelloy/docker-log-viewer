// Test script to inject a mock schema for demonstration purposes
export const mockSchema = {
    queryType: { name: "Query" },
    mutationType: { name: "Mutation" },
    types: [
        {
            kind: "OBJECT",
            name: "Query",
            description: "The root query type",
            fields: [
                {
                    name: "user",
                    description: "Get a user by ID",
                    args: [{
                        name: "id",
                        description: "The user ID",
                        type: { kind: "NON_NULL", ofType: { kind: "SCALAR", name: "ID" } }
                    }],
                    type: { kind: "OBJECT", name: "User" }
                },
                {
                    name: "users",
                    description: "Get all users",
                    args: [{
                        name: "limit",
                        description: "Maximum number of users to return",
                        type: { kind: "SCALAR", name: "Int" }
                    }],
                    type: { kind: "LIST", ofType: { kind: "OBJECT", name: "User" } }
                },
                {
                    name: "posts",
                    description: "Get all posts",
                    args: [],
                    type: { kind: "LIST", ofType: { kind: "OBJECT", name: "Post" } }
                }
            ]
        },
        {
            kind: "OBJECT",
            name: "Mutation",
            description: "The root mutation type",
            fields: [
                {
                    name: "createUser",
                    description: "Create a new user",
                    args: [{
                        name: "input",
                        description: "User input data",
                        type: { kind: "NON_NULL", ofType: { kind: "INPUT_OBJECT", name: "CreateUserInput" } }
                    }],
                    type: { kind: "OBJECT", name: "User" }
                },
                {
                    name: "updateUser",
                    description: "Update an existing user",
                    args: [
                        {
                            name: "id",
                            description: "User ID",
                            type: { kind: "NON_NULL", ofType: { kind: "SCALAR", name: "ID" } }
                        },
                        {
                            name: "input",
                            description: "User update data",
                            type: { kind: "NON_NULL", ofType: { kind: "INPUT_OBJECT", name: "UpdateUserInput" } }
                        }
                    ],
                    type: { kind: "OBJECT", name: "User" }
                },
                {
                    name: "deleteUser",
                    description: "Delete a user",
                    args: [{
                        name: "id",
                        description: "User ID to delete",
                        type: { kind: "NON_NULL", ofType: { kind: "SCALAR", name: "ID" } }
                    }],
                    type: { kind: "SCALAR", name: "Boolean" }
                }
            ]
        },
        {
            kind: "OBJECT",
            name: "User",
            description: "A user in the system",
            fields: [
                { name: "id", description: "User ID", args: [], type: { kind: "NON_NULL", ofType: { kind: "SCALAR", name: "ID" } } },
                { name: "name", description: "User's full name", args: [], type: { kind: "SCALAR", name: "String" } },
                { name: "email", description: "User's email address", args: [], type: { kind: "SCALAR", name: "String" } },
                { name: "posts", description: "Posts created by this user", args: [], type: { kind: "LIST", ofType: { kind: "OBJECT", name: "Post" } } }
            ]
        },
        {
            kind: "OBJECT",
            name: "Post",
            description: "A blog post",
            fields: [
                { name: "id", description: "Post ID", args: [], type: { kind: "NON_NULL", ofType: { kind: "SCALAR", name: "ID" } } },
                { name: "title", description: "Post title", args: [], type: { kind: "SCALAR", name: "String" } },
                { name: "content", description: "Post content", args: [], type: { kind: "SCALAR", name: "String" } },
                { name: "author", description: "Post author", args: [], type: { kind: "OBJECT", name: "User" } }
            ]
        }
    ]
};
