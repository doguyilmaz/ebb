import EbbCore

enum EbbConnection {
    case opening
    case ready(EbbClient)
    case failed(String)
}
