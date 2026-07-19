import 'package:app/models/conversation.dart';
import 'package:app/models/user.dart';
import 'package:flutter/material.dart';

class ConversationsScreen extends StatefulWidget {
  const ConversationsScreen({super.key});

  @override
  State<ConversationsScreen> createState() => _ConversationsScreenState();
}

class _ConversationsScreenState extends State<ConversationsScreen> {
  // TODO: this is for testing only, remove later...
  final List<Conversation> _conversations = [
    Conversation(
      participants: <User>[
        User(
          email: "johndoe@example.com",
          username: "johndoe",
          firstName: "John",
          lastName: "Doe",
        ),
        User(
          email: "janedoe@example.com",
          username: "janedoe",
          firstName: "Jane",
          lastName: "Doe",
        ),
      ],
    ),
  ];

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Conversations')),
      body: ListView.builder(
        itemCount: _conversations.length,
        itemBuilder: (context, index) {
          return ListTile(
            leading: Text('$index'),
            title: Text(_conversations[index].name),
          );
        },
      ),
    );
  }
}
