import 'package:app/models/user.dart';

class Conversation {
  Conversation({this._name, required this._participants});

  final String? _name;
  final List<User> _participants;

  String get name => _name ?? _nameFromParticipants();

  String _nameFromParticipants() {
    return _participants.map((e) => e.firstName).join(", ");
  }
}
