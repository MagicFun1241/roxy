register_plugin('ratelimit', {
    optionsSchema: {
        window: {
            type: 'string'
        }
    }
})

register_middleware("ratelimit", (ctx) => {
    console.log(ctx.options.window)

    console.log(ctx.ip)
    ctx.status(400)
    ctx.reply({
        message: 'TODO'
    })
})