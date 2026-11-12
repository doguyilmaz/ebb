import SwiftUI

struct EmptyState<Actions: View>: View {
    let title: String
    let message: String
    let symbol: String
    @ViewBuilder let actions: () -> Actions

    var body: some View {
        VStack(spacing: 14) {
            Image(systemName: symbol).font(.system(size: 36)).foregroundStyle(.secondary)
            Text(title).font(.title2.weight(.semibold))
            Text(message).foregroundStyle(.secondary).multilineTextAlignment(.center).frame(maxWidth: 440)
            actions()
        }
        .padding(32).frame(maxWidth: .infinity, maxHeight: .infinity)
    }
}
