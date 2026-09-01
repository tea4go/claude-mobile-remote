@Component
export struct SelfCheckPanel {
  @Prop results: string[] = []
  @Prop error: string = ''

  build() {
    Column({ space: 8 }) {
      Text('Self Check')
        .fontSize(18)
        .fontWeight(FontWeight.Medium)

      if (this.error.length > 0) {
        Text(this.error)
          .fontSize(14)
          .fontColor('#c0392b')
      } else if (this.results.length > 0) {
        ForEach(this.results, (item: string) => {
          Text(`- ${item}`)
            .fontSize(14)
            .width('100%')
        }, (item: string) => item)
      } else {
        Text('- self-check-pending')
          .fontSize(14)
      }
    }
    .id('self-check-summary')
    .padding(16)
    .width('100%')
    .alignItems(HorizontalAlign.Start)
    .backgroundColor('#f4f6f8')
    .borderRadius(12)
  }
}
